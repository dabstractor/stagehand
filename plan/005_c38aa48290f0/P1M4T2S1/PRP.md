name: "P1.M4.T2.S1 — git-alias integration target"
description: |
  Implement the FIRST concrete `integrate.Entry` target — **`git-alias`** (PRD §9.21 FR-I4, FR-I6, §15.3).
  Registers the `git stagehand` alias in git's GLOBAL config so `git stagehand` runs stagehand. The edit is
  delegated to git itself (`git config --global alias.<name> '!stagehand'` / `--unset`), so the FR-I3
  no-mangle FILE machinery (parse/backup/atomic/validate) is unnecessary — but FR-I3c (preview + confirm)
  still applies: install shows the exact command + resulting usage, a `y/N` prompt (`--yes` skips), and
  surfaces an existing `alias.<name>` with a DIFFERENT value before overwriting; remove only unsets when the
  current value is ours (sans-`!` == `stagehand`), refusing (NoChange + note) a foreign alias (FR-I6).
  `--alias-name <n>` overrides the default name `stagehand` (applies to install AND remove). Detection needs
  only git (FR-I2). Adds the three repo-independent global-config methods (`ConfigGlobalGet/Set/Unset`) to
  the existing `Git` interface — the "existing internal/git exec seam" the work item names. Ships its own
  preview+confirm (does NOT call `protocol.Apply`) via S1's shared `ConfirmFunc`/`DefaultConfirm`.
  **Consumer of S1 (protocol.Outcome/ConfirmFunc/DefaultConfirm) + S2 (integrate.Entry/Registry/Status/
  InstallOptions/InstallResult + defaultEntries seam + dispatch); co-resident with T2.S2 (lazygit).** Adds
  the docs/cli.md git-alias target section (Mode A). NO new third-party dependency (yaml.v3 is T2.S2's).

---

## Goal

**Feature Goal**: Ship the `git-alias` integration target end-to-end so that `stagehand integrate install
git-alias` makes `git stagehand` work, and `stagehand integrate remove git-alias` cleanly undoes it — with
the same idempotent, never-clobber, preview-and-confirm discipline as the rest of `integrate`, but
implemented by delegating the actual `.gitconfig` write to git itself. A conflicting `alias.<name>` set to
something other than stagehand's is always surfaced before it is overwritten (install) and never silently
removed (remove). The target is fully isolated in tests (a temp `GIT_CONFIG_GLOBAL` file — the real global
config is never touched) and is the first concrete `Entry` to light up S2's otherwise-empty registry.

**Deliverable**:
1. `internal/git/git.go` — three new `Git` interface methods + `gitRunner` impls: `ConfigGlobalGet`,
   `ConfigGlobalSet`, `ConfigGlobalUnset` (repo-independent; `-C workDir` is a no-op for `--global`).
2. `internal/git/configglobal_test.go` — the methods' test file (hookspath_test.go style), isolated via
   `t.Setenv("GIT_CONFIG_GLOBAL", <tmpfile>)`.
3. `internal/cmd/integrate_gitalias.go` — `gitAliasEntry` (implements `integrate.Entry`: Name/Detect/
   ConfigPath/Status/Install/Remove) + the `--alias-name` flag (local on install AND remove) + init()
   registration.
4. `internal/cmd/integrate.go` (S2's file) — ONE edit: `defaultEntries` returns
   `[]integrate.Entry{ &gitAliasEntry{...} }` (was `nil`).
5. `internal/cmd/integrate_test.go` (S2's file) — ONE edit: `resetIntegrateFlags` also resets `flagAliasName`
   + its Changed bit.
6. `internal/cmd/integrate_gitalias_test.go` — gitAliasEntry matrix (real git via `git.New(<tmpdir>)` +
   `GIT_CONFIG_GLOBAL` isolation; fixed-bool Confirm) + Execute-level wiring for `--alias-name`.
7. `docs/cli.md` — the git-alias target subsection (Mode A): `--alias-name`, the conflicting-alias behavior,
   `git config` delegation, what `integrate list`/`status` show for it.

**Success Definition**: `integrate list` shows `git-alias` DETECTED ✓, STATUS not-installed/installed/foreign,
CONFIG = the global gitconfig path. `integrate install git-alias` → (preview: `git config --global
alias.stagehand '!stagehand'` + `git stagehand`) → confirm y → `git stagehand` works; re-run → "already
installed" (NoChange). A foreign `alias.stagehand` is surfaced before overwrite (Updated after confirm). A
foreign alias on `remove` is refused (NoChange + note; never removed). `--alias-name foo` manages
`alias.foo` instead. Tests never touch the real global config. `go build ./...`, `go test ./...`,
`go vet ./...`, `golangci-lint run`, `gofmt -l` all green; `go.mod` UNCHANGED.

## User Persona

**Target User**: the "plan-holder" (PRD §7.1) who types `git <thing>` all day and wants `git stagehand` as a
first-class git subcommand (the originating `commit-pi` habit, PRD §2.1/§16.3). They run
`stagehand integrate install git-alias` once; thereafter `git stagehand` is in their muscle memory next to
`git commit`. Their one fear: "did stagehand clobber an alias I already defined?" — the conflicting-alias
surfacing + never-remove-foreign contract is the answer.

**Use Case**: `stagehand integrate install git-alias` → confirm → `git stagehand` runs stagehand. Later
`stagehand integrate remove git-alias` restores the global config to its pre-stagehand state for that alias.

**User Journey**: `integrate list` (see git-alias, DETECTED ✓, not installed) → `integrate install
git-alias` (see command + usage → `y`) → use `git stagehand` → later `integrate remove git-alias` (confirm →
gone). `--yes` for scripts; `--alias-name ci` to install as `git ci`.

**Pain Points Addressed**: no hand-editing of `.gitconfig` (git itself writes it, idempotently); a foreign
alias is never silently destroyed; the alias name is overridable without editing files.

## Why

- **PRD §9.21 FR-I4**: `git-alias` delegates to `git config --global alias.<name> '!stagehand'`; git does
  the `.gitconfig` edit so the FR-I3 file machinery is unnecessary, but the command + resulting alias are
  shown and confirmed, and a conflicting `alias.<name>` is surfaced before overwriting.
- **PRD §9.21 FR-I6**: uninstall symmetry — `git config --global --unset alias.<name>` only when its value
  is ours.
- **PRD §9.21 FR-I2**: `git-alias` requires only git itself (detection always passes when git is present).
- **delta_prd.md R4** (line 47/67-68): "`git-alias` target delegates the file edit to `git config` via the
  existing git exec wrapper (still previewed/confirmed; conflicting alias surfaced)."
- **architecture/external_deps.md §7** (VERIFIED): the exact alias mechanics — `!` aliases run as shell
  commands from the repo toplevel; read-back prints the value INCLUDING `!` (strip when comparing ours);
  `--get` exit 1 = unset; `--unset` exit 5 = not set.
- **Scope fences**: CONSUMES S1's `integrate.Outcome`/`ConfirmFunc`/`DefaultConfirm` and S2's
  `integrate.Entry`/`Registry`/`Status`/`InstallOptions`/`InstallResult` + the `defaultEntries` seam +
  dispatch; PROVIDES the first concrete target that lights up S2's surface. Does NOT add yaml.v3 (T2.S2),
  does NOT re-implement the protocol engine (S1) or the command surface (S2), does NOT touch README (P1.M7).

## What

A concrete `integrate.Entry` ("git-alias") whose install/remove delegate the `.gitconfig` write to
`git config --global`, wrapped in S2's shared preview+confirm. It is registered into S2's `defaultEntries`
seam so `integrate list|install|remove` drive it with zero further cmd-layer work. The git config plumbing
goes through three new repo-independent methods on the existing `Git` interface.

### Success Criteria

- [ ] `internal/git/git.go`: `ConfigGlobalGet(ctx,key)(value,found,err)`, `ConfigGlobalSet(ctx,key,value)error`,
      `ConfigGlobalUnset(ctx,key)(found,err)` added to the `Git` interface + impls on `gitRunner` via `run()`;
      exit-code semantics: `--get` exit 1 = not-found (found=false), `--unset` exit 5 = not-set (found=false),
      everything else non-zero = wrapped error; value trimmed; repo-independent (`-C workDir` no-op for `--global`).
- [ ] `internal/git/configglobal_test.go`: get-found/get-missing/set/unset/unset-missing + isolation via
      `t.Setenv("GIT_CONFIG_GLOBAL", <tmpfile>)` (real global config untouched); mirrors hookspath_test.go.
- [ ] `internal/cmd/integrate_gitalias.go`: `gitAliasEntry` implements all six `Entry` methods (see Blueprint);
      `--alias-name` flag (local on `integrateInstallCmd` AND `integrateRemoveCmd`, shared `flagAliasName`,
      default `""`→`"stagehand"`); registered in `init()`. install/remove own their preview+confirm
      (build a command+usage+optional-conflict preview string; call the shared ConfirmFunc; honor `Yes`).
      Returns `Outcome` per the mapping table; `Backup` always `""`.
- [ ] `internal/cmd/integrate.go` (S2's): `defaultEntries` returns `[]integrate.Entry{ &gitAliasEntry{...} }`.
- [ ] `internal/cmd/integrate_test.go` (S2's): `resetIntegrateFlags` resets `flagAliasName` + Changed bit.
- [ ] `internal/cmd/integrate_gitalias_test.go`: full matrix (install-creates, idempotent-already-ours=NoChange,
      foreign-install=Updated-after-confirm, conflict-surfaced-before-overwrite, decline, remove-ours=Removed,
      remove-foreign=NoChange+note, remove-unset=NoChange, `--alias-name` custom name, Detect PATH gate,
      Status three states, ConfigPath) — all isolated via `GIT_CONFIG_GLOBAL`; + Execute wiring for `--alias-name`.
- [ ] `docs/cli.md`: git-alias target section (`--alias-name`, conflicting-alias behavior, `git config`
      delegation, `integrate list`/status representation).
- [ ] `go build ./...` + `go test ./...` + `go vet ./...` + `golangci-lint run` + `gofmt -l` clean;
      `go.mod` UNCHANGED (no new `require` — yaml.v3 is T2.S2's).

## All Needed Context

### Context Completeness Check

_This PRP names the EXACT three `Git.ConfigGlobal*` signatures + their git exit-code semantics (verified:
`--get` exit 1, `--unset` exit 5), the EXACT alias "ours" test (strip leading `!`, compare to `stagehand`),
the EXACT Outcome mapping per (action × current-state), the EXACT Entry method set from S2 + the
InstallOptions/Result shapes from S1, the env-passthrough test-isolation fact (`run()` inherits parent env
so `t.Setenv("GIT_CONFIG_GLOBAL",…)` isolates), the `defaultEntries`/`resetIntegrateFlags` single-line edits
to S2's files, the `--alias-name` flag placement (local on install AND remove, hook.go `--strict` precedent),
the preview+confirm reuse of `ConfirmFunc`/`DefaultConfirm`, the docs/cli.md placement, and the scope fences.
An implementer with no prior codebase knowledge can build it from this document + codebase access._

### Documentation & References

```yaml
- docfile: plan/005_c38aa48290f0/P1M4T2S1/research/codebase-patterns.md
  why: Condensed research — the verified git alias mechanics (§7), the central design decision (Git.ConfigGlobal*
       methods, not a new exec helper — with the delta_prd "existing git exec wrapper" + HooksPath precedent),
       the Entry contract from S2, the defaultEntries seam, the Outcome mapping table, the env-passthrough test
       isolation, and the file conventions.
  section: all
  critical: |
    git-alias delegates the .gitconfig WRITE to `git config --global` — it does NOT use protocol.Apply (FR-I4).
    But FR-I3c preview+confirm still applies: build a preview string (command + usage + optional conflict),
    call the shared ConfirmFunc (nil ⇒ DefaultConfirm, TTY-gated; --yes bypassed by the caller). "Ours" =
    TrimPrefix(storedValue, "!") == "stagehand". Tests isolate via t.Setenv("GIT_CONFIG_GLOBAL", tmpfile)
    (run() inherits parent env; GIT_CONFIG_GLOBAL REPLACES ~/.gitconfig → full isolation).

- docfile: plan/005_c38aa48290f0/P1M4T1S2/PRP.md
  why: THE CONTRACT — the `Entry` interface (Name/Detect/ConfigPath/Status/Install/Remove), `Status` enum
       (NotInstalled/Installed/Foreign + String tokens), `InstallOptions`/`RemoveOptions` {Yes,Out,Confirm},
       `InstallResult`/`RemoveResult` {Outcome,Target,Path,Backup}, `defaultEntries` seam, `dispatchInstall`/
       `dispatchRemove` (per-target Detect gate → Install/Remove; Decline/NoChange not errors; formatInstallResult),
       `resetIntegrateFlags`, the cobra `integrate` group (no-op PersistentPreRunE, persistent --yes), and the
       --alias-name flag being T2.S1's (local on install+remove). Treat as authoritative.
  section: "Data models and structure; Implementation Tasks; Integration Points (PROVIDES)"
  critical: |
    S2's dispatchInstall/dispatchRemove call Detect BEFORE Install/Remove and wrap errors via exitcode.New.
    git-alias must return the right integrate.Outcome so S2's formatInstallResult prints the right verb.
    git-alias does NOT call dispatch itself — it implements Entry; S2's already-written dispatch drives it.
    resetIntegrateFlags (S2) resets flagIntegrateYes — T2.S1 APPENDS the flagAliasName reset.

- docfile: plan/005_c38aa48290f0/P1M4T1S1/PRP.md
  why: THE upstream contract — `integrate.Outcome` (Created/Updated/Removed/Declined/NoChange + String),
       `integrate.ConfirmFunc = func(out io.Writer, path, diff string) bool`, `integrate.DefaultConfirm`
       (writes diff; non-TTY stdin auto-declines; --yes bypassed by caller). git-alias reuses these verbatim
       (Outcome for results; ConfirmFunc for its own preview+confirm). InstallResult.Outcome IS integrate.Outcome.
  section: "Data models and structure; Integration Points (PROVIDES)"
  critical: |
    Do NOT re-invent an Outcome enum or a confirm prompt — reuse integrate.Outcome / ConfirmFunc / DefaultConfirm.
    git-alias's "preview" is a command+usage string passed as the `diff` arg to ConfirmFunc — DefaultConfirm
    prints it then "Apply changes to <configpath>? [y/N]". Backup is ALWAYS "" for git-alias (git owns the file).

- docfile: plan/005_c38aa48290f0/architecture/external_deps.md
  why: §7 (VERIFIED) — the git alias mechanics this subtask implements: `git config --global alias.<name>
       '!stagehand'`; read-back via `--get` prints `!stagehand` (strip `!` when comparing ours); exit 1 = unset;
       `--unset` exit 5 = not set. §8 — the test-isolation precedent (GIT_CONFIG_GLOBAL / GIT_CONFIG_SYSTEM).
  section: "## 7. git alias mechanics (gates FR-I4)"
  critical: |
    The stored value INCLUDES the leading `!`. To test "is it ours": value == "!stagehand" OR equivalently
    strings.TrimPrefix(value,"!") == "stagehand". `--get` uses exit code 1 (NOT stdout emptiness) as the
    "missing" signal; `--unset` uses exit code 5. Use `--get` (portable), NOT the 2.46+ `config get` subcommand.

- docfile: plan/005_c38aa48290f0/prd_snapshot.md  (and PRD.md §9.21 / §15.3)
  why: §9.21 FR-I4 (git-alias: delegate to git config; preview+confirm; conflicting alias surfaced before
       overwrite), FR-I6 (uninstall symmetry: --unset only when ours), FR-I2 (git-alias needs only git),
       FR-I1 (list columns: detected/config-path/status), §15.3 (subcommand reference), §15.4 (exit codes).
  section: "§9.21 (FR-I1/I2/I4/I6), §15.3, §15.4"

- docfile: plan/005_c38aa48290f0/delta_prd.md
  why: R4 line 47 — "git-alias target delegates the file edit to git config via the existing git exec wrapper";
       R4 line 67-68 — "still previewed/confirmed; conflicting alias surfaced"; line 69 — docs Mode A docs/cli.md.
  section: "R4 — Tool integrations"

- file: internal/git/git.go
  why: THE exec seam. (a) The `Git` INTERFACE (top of file) — add ConfigGlobalGet/Set/Unset here. (b) `run()`
       (~L260) — the helper the impls call: LookPath → `-C workDir` + args ([]string, NO shell, PRD §19) →
       separate stdout/stderr buffers → errors.As(ExitError) with err==nil + code for non-zero exits. (c) HooksPath
       (L1331-1363) — the DIRECT precedent for a new Git method added for an integration feature (P1.M3 hook).
       (d) gitConfigGet's exit-code branching is mirrored in config/git.go (cited below).
  pattern: |
    impl := func: stdout,stderr,code,err := g.run(ctx, g.workDir, "config", "--global", "--get", key); if err!=nil
    {return ...err}; switch code {case 0: return trimmed, true, nil; case 1: return "", false, nil; default:
    return "", false, fmt.Errorf("git config --global --get %s: exit %d: %s", key, code, strings.TrimSpace(stderr))}.
  gotcha: |
    run() does NOT set cmd.Env → the child inherits the PARENT env → t.Setenv("GIT_CONFIG_GLOBAL", tmpfile)
    propagates (full test isolation). The `-C workDir` is a harmless no-op for --global scope (git config
    --global targets ~/.gitconfig / $GIT_CONFIG_GLOBAL regardless of cwd) — pass workDir as-is (git.New(cwd)).

- file: internal/git/hookspath_test.go
  why: THE test-file convention for a new Git method — t.TempDir() + the method's dedicated _test.go. NOTE:
       its gitDo sets cmd.Env=minGitEnv() (PATH+HOME) for RAW exec.Command helpers; the Git INTERFACE methods
       use run() which INHERITS parent env, so ConfigGlobal* tests use t.Setenv (NOT minGitEnv) for GIT_CONFIG_GLOBAL.
  pattern: "repo := t.TempDir(); g := New(repo); got, err := g.<Method>(context.Background()); assert."
  gotcha: "Do NOT call minGitEnv for ConfigGlobal* tests — you WANT the inherited env so t.Setenv's
           GIT_CONFIG_GLOBAL reaches git. (minGitEnv is for the raw-exec gitDo helper, not the Git interface.)"

- file: internal/config/git.go
  why: THE git-config exit-code precedent (gitConfigGet, ~L40): `git config --get <key>` exit 0 = found,
       exit 1 = missing (found=false, NOT an error), else wrapped error incl. stderr. ConfigGlobalGet mirrors
       this EXACTLY (adds --global). It has its OWN gitExec ONLY to avoid an import cycle — integrate has none,
       so git-alias uses the shared Git interface instead.
  pattern: "switch code {case 0: found; case 1: missing; default: error}."
  gotcha: "config/git.go reads REPO-local config (no --global). git-alias reads/writes GLOBAL (--global). Same
           exit-code discipline, different scope."

- file: internal/cmd/hook.go
  why: THE cmd-layer precedent for (a) a per-target LOCAL flag (--strict/--print local to hookInstallCmd) — the
       template for where --alias-name goes; (b) exitcode.New routing + "Installed"/"Updated"/"No X to remove"
       messaging style; (c) foreign-refusal messaging (hook prints the manual-invocation line; git-alias prints
       the conflicting value instead, since overwriting IS allowed after confirm).
  pattern: "cmd.Flags().BoolVar(&flag, \"--name\", default, \"...\") in init(); RunE reads the flag; errors → exitcode.New."
  gotcha: "--alias-name is needed by BOTH install and remove (you remove the alias by name). Register it as a
           LOCAL flag on integrateInstallCmd AND integrateRemoveCmd sharing ONE var (flagAliasName). hook's
           --strict is install-only; git-alias differs."

- file: internal/cmd/integrate.go   (S2's file — created in parallel by P1.M4.T1.S2)
  why: THE file T2.S1 EDITS minimally: (1) `defaultEntries` (S2 ships `return nil`) → return
       []integrate.Entry{ &gitAliasEntry{...} }; (2) it defines resetIntegrateFlags (T2.S1 appends the
       --alias-name reset). S2 also defines the cobra integrateCmd/integrateInstallCmd/integrateRemoveCmd vars
       + the persistent --yes flag — T2.S1 REFERENCES integrateInstallCmd/integrateRemoveCmd in its own init()
       (same package) to register --alias-name. Do NOT rewrite S2's file — append/extend.
  pattern: "defaultEntries builds the entry from the resolved flag + git.New(cwd) (cwd from os.Getwd, '.' fallback)."
  gotcha: "defaultEntries is called INSIDE runIntegrateList/Install/Remove (per command, after flag parse), so
           flagAliasName is already populated when gitAliasEntry is constructed. os.Getwd() may fail (rare) →
           use '.' (global config ignores cwd)."

- file: internal/cmd/providers.go   (and providers_test.go)
  why: exitcode.New routing + OutOrStdout/ErrOrStderr + SilenceErrors/Usage conventions; providers_test.go's
       saveRootState/restoreRootState/SetArgs/Execute test style (referenced by S2's integrate_test.go; T2.S1's
       Execute-level --alias-name wiring test follows it).
  pattern: "_,o,e,r := saveRootState(t); defer restoreRootState(t,nil,o,e,r); rootCmd.SetArgs(...); Execute(ctx)."
  gotcha: "Primary git-alias tests call the gitAliasEntry methods DIRECTLY (real git + GIT_CONFIG_GLOBAL) —
           reserve Execute-level tests for --alias-name parsing/wiring only."

- file: internal/exitcode/exitcode.go
  why: exitcode.New(exitcode.Error, err) for the (rare) hard error path (e.g. git binary missing at Install
       time, ConfigGlobalSet fails). S2's dispatch already wraps Entry errors; git-alias returns plain errors
       from its methods and S2 routes them. git-alias itself does NOT call exitcode (it's the Entry impl).
  pattern: "git-alias returns plain fmt.Errorf from Entry methods; S2's dispatchInstall/Remove wrap + exit 1."
  gotcha: "Decline/NoChange are NOT errors (exit 0) — they're communicated via Outcome, returned with nil err."

- file: go.mod
  why: confirms deps are cobra/pflag/go-toml/v2 ONLY. yaml.v3 is ABSENT — this subtask adds NOTHING to go.mod.
  gotcha: "git-alias needs only stdlib (os, os/exec, path/filepath, strings, fmt) + internal/git + internal/integrate.
           If you reach for yaml/toml, STOP — that's T2.S2."
```

### Current Codebase tree (relevant slice)

```bash
internal/
  git/
    git.go                      # Git interface (TOP) + all gitRunner impls (HooksPath @ L1340). ADD ConfigGlobal*.
    git_test.go                 # REF — run()/New test style, initRepo helper (do NOT edit)
    hookspath_test.go           # REF — the per-method test FILE convention (do NOT edit)
    revparse_test.go            # REF — minGitEnv (for raw-exec helpers; NOT for Git-interface tests) (do NOT edit)
  config/git.go                 # REF — gitConfigGet exit-code precedent (--get: 0 found / 1 missing) (do NOT edit)
  integrate/                    # PACKAGE from S1/S2 (siblings: protocol.go S1, registry.go S2)
    protocol.go                 # S1 — Outcome/ConfirmFunc/DefaultConfirm (CONSUME) (do NOT edit)
    registry.go                 # S2 — Entry/Status/InstallOptions/InstallResult/Registry (CONSUME) (do NOT edit)
  cmd/
    integrate.go                # S2's — defaultEntries seam + integrateCmd/Install/Remove + resetIntegrateFlags.
                                #   T2.S1 EDITS: defaultEntries body + ONE resetIntegrateFlags line.
    integrate_test.go           # S2's — T2.S1 EDITS: resetIntegrateFlags --alias-name reset line.
    hook.go                     # REF — local-flag-on-install precedent (do NOT edit)
    providers.go                # REF — exitcode/OutOrStdout conventions (do NOT edit)
    integrate_gitalias.go       # NEW (T2.S1) — gitAliasEntry + --alias-name flag + init().
    integrate_gitalias_test.go  # NEW (T2.S1) — gitAliasEntry matrix + --alias-name wiring.
  exitcode/exitcode.go          # REF (do NOT edit)
docs/
  cli.md                        # EDIT (Mode A) — git-alias target section (EXTENDS S2's integrate group section)
go.mod                          # UNCHANGED (no new require)
```

### Desired Codebase tree with files to be added and responsibility of file

```bash
internal/git/
  git.go                       # + 3 Git interface methods (ConfigGlobalGet/Set/Unset) + gitRunner impls (run()-based).
  configglobal_test.go         # NEW — get/set/unset + isolation (GIT_CONFIG_GLOBAL); hookspath_test.go style.
internal/cmd/
  integrate_gitalias.go        # NEW — gitAliasEntry (all 6 Entry methods) + --alias-name flag (local on install
                               #   AND remove) + init() registering it. Owns its preview+confirm (no protocol.Apply).
  integrate_gitalias_test.go   # NEW — full gitAliasEntry matrix (real git + GIT_CONFIG_GLOBAL isolation) + wiring.
  integrate.go                 # EDIT — defaultEntries returns &gitAliasEntry{...} (was nil).
  integrate_test.go            # EDIT — resetIntegrateFlags resets flagAliasName + Changed bit.
docs/
  cli.md                       # EDIT — git-alias target subsection under S2's `integrate` group section.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (git-alias does NOT use protocol.Apply): FR-I4 delegates the .gitconfig edit to `git config`.
// git-alias owns its own Install/Remove: build a preview string (command + usage + optional conflict), call
// the shared ConfirmFunc (opts.Confirm, nil⇒DefaultConfirm) honoring opts.Yes, then run ConfigGlobalSet/Unset.
// InstallResult.Backup is ALWAYS "" (git owns the file; no backup machinery). This is the file-edit vs
// command-delegate asymmetry S2's Entry interface was designed to accommodate.

// CRITICAL (the "ours" test strips the leading `!`): `git config --global --get alias.<name>` prints the
// stored value INCLUDING the `!` (e.g. "!stagehand"). An alias is OURS iff strings.TrimPrefix(value, "!") ==
// "stagehand". Compare the COMMAND part, not the alias NAME (the name can be overridden via --alias-name but
// the command is always "stagehand"). external_deps.md §7 (VERIFIED).

// CRITICAL (--get exit 1 = unset, --unset exit 5 = not-set — NOT errors): ConfigGlobalGet returns found=false
// on exit 1 (nil err); ConfigGlobalUnset returns found=false on exit 5 (nil err). Branch on the exit CODE,
// never on stdout emptiness (mirrors config/git.go gitConfigGet + internal/git run()'s convention). Use `--get`
// (portable), NOT the 2.46+ `config get` subcommand form.

// CRITICAL (test isolation via inherited env): run() does NOT set cmd.Env → the child git INHERITS the parent
// env. So t.Setenv("GIT_CONFIG_GLOBAL", <tmpfile>) propagates to the subprocess, and GIT_CONFIG_GLOBAL (when
// set) REPLACES ~/.gitconfig → the real global config is NEVER read or written. Do NOT use minGitEnv for the
// Git-interface tests (it would strip GIT_CONFIG_GLOBAL); it's only for the raw-exec gitDo helper. Construct
// the entry with git.New(t.TempDir()) — NO repo init needed (git config --global works anywhere).

// CRITICAL (-C workDir is a harmless no-op for --global): the Git interface is repo-bound (-C workDir), but
// `git config --global` targets the user's global file regardless of cwd. So git.New(cwd) works for git-alias
// even outside a repo (integrate SKIPS config.Load / works outside a repo — S2's no-op PersistentPreRunE).
// cwd from os.Getwd(); on the rare Getwd error, fall back to "." (git -C . config --global works).

// GOTCHA (--alias-name on install AND remove): hook's --strict is install-only; git-alias's --alias-name must
// apply to BOTH (you remove the alias by name). Register it as a LOCAL flag on integrateInstallCmd AND
// integrateRemoveCmd sharing ONE var (flagAliasName), in integrate_gitalias.go's init(). Default "" → resolved
// to "stagehand" inside the entry (so `list`/`status` report the resolved name, not "").

// GOTCHA (defaultEntries is called per-command, after flag parse): runIntegrateList/Install/Remove call
// defaultEntries() fresh each time (S2's design), so flagAliasName is populated when gitAliasEntry is built.
// Do NOT construct the entry at package-init time (flags not parsed yet).

// GOTCHA (Detect is git on $PATH — FR-I2): git-alias needs only git. Detect = exec.LookPath("git") (return
// error if missing). Test absence via t.Setenv("PATH","") (mirrors internal/git TestRun_LookPathFailure). Do
// NOT make Detect require a repo or run a repo-bound git command.

// GOTCHA (foreign-on-install is OVERWRITE-after-surfacing; foreign-on-remove is REFUSE): these differ!
// Install over a foreign alias: surface the current value in the preview, then overwrite after confirm (Updated).
// Remove of a foreign alias: NEVER unset (FR-I6 "only when the value is ours") → NoChange + a stderr note.
// (Contrast hook: hook REFUSES to overwrite a foreign hook; git-alias OVERWRITES after surfacing — FR-I4.)

// GOTCHA (git-alias is the Entry impl; S2's dispatch already exists): do NOT re-implement dispatchInstall/
// dispatchRemove/printIntegrateList — S2 wrote them. git-alias implements Entry; S2's dispatch drives it. The
// only edits to S2's files are defaultEntries (return the entry) + resetIntegrateFlags (reset --alias-name).

// GOTCHA (ConfigPath is display-only + best-effort): the `list` CONFIG column needs a path. Resolve:
// GIT_CONFIG_GLOBAL env (if set+non-empty, absolute-ized) else $HOME/.gitconfig (error if HOME unset). This is
// correct for the common case + deterministic in tests (GIT_CONFIG_GLOBAL set). git also checks
// $XDG_CONFIG_HOME/git/config — document this as a known limitation; do not over-engineer (no reliable single
// git command prints the global config PATH when the file is empty).

// GOTCHA (no new dependency): git-alias imports ONLY stdlib + internal/git + internal/integrate. yaml.v3 is
// T2.S2's. go.mod MUST stay unchanged.
```

## Implementation Blueprint

### Data models and structure

```go
// === internal/git/git.go — add to the Git INTERFACE + impls on gitRunner ===

// ConfigGlobalGet reads a key from git's GLOBAL config (not repo-local) via
// `git config --global --get <key>` (PRD §9.21 FR-I4). Exit-code semantics (mirrors config/git.go
// gitConfigGet): 0 = found (trimmed value); 1 = not found (found=false, NOT an error); else wrapped error
// incl. stderr. Repo-independent: the `-C workDir` from run() is a harmless no-op for `--global` scope
// (git config --global targets ~/.gitconfig / $GIT_CONFIG_GLOBAL regardless of cwd). Used by integrate
// git-alias to read back alias.<name> — the stored value INCLUDES the leading `!` for shell aliases, so
// callers strip it when comparing whether the alias is stagehand's (external_deps.md §7).
ConfigGlobalGet(ctx context.Context, key string) (value string, found bool, err error)

// ConfigGlobalSet writes a key/value to git's GLOBAL config via `git config --global <key> <value>`
// (PRD §9.21 FR-I4). git performs the .gitconfig edit itself (so the FR-I3 file machinery is unnecessary).
// value is passed as a SINGLE argv element (NEVER sh -c — PRD §19), so a value like "!stagehand" is stored
// verbatim with its `!`. Non-zero exit ⇒ wrapped error incl. stderr.
ConfigGlobalSet(ctx context.Context, key, value string) error

// ConfigGlobalUnset removes a key from git's GLOBAL config via `git config --global --unset <key>`
// (PRD §9.21 FR-I6). Returns found=false when the key was not present (git exit 5 — NOT an error);
// found=true + nil when removed. The caller (git-alias) MUST first verify the value is ours before
// calling this (FR-I6: only unset when the current value is stagehand's).
ConfigGlobalUnset(ctx context.Context, key string) (found bool, err error)

// === internal/cmd/integrate_gitalias.go — the gitAliasEntry Entry impl ===
package cmd // import "github.com/dustin/stagehand/internal/cmd"

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dustin/stagehand/internal/git"
	"github.com/dustin/stagehand/internal/integrate"
)

const (
	gitAliasTarget      = "git-alias"   // Entry.Name()
	defaultAliasName    = "stagehand"   // default alias name → `git stagehand`
	stagehandAliasValue = "!stagehand"  // the stored value (incl. `!`); command part is "stagehand"
)

var flagAliasName string // --alias-name (local on integrateInstallCmd AND integrateRemoveCmd)

func init() {
	// Register --alias-name on BOTH leaves (you remove the alias by name). Shared var. Default "" →
	// resolved to "stagehand" inside the entry. hook.go's --strict is the local-flag precedent.
	integrateInstallCmd.Flags().StringVar(&flagAliasName, "alias-name", "",
		"Override the git alias name (default: stagehand → `git stagehand`)")
	integrateRemoveCmd.Flags().StringVar(&flagAliasName, "alias-name", "",
		"Override the git alias name to remove (default: stagehand)")
}

// gitAliasEntry implements integrate.Entry for the git-alias target (PRD §9.21 FR-I4/I6). It delegates the
// .gitconfig WRITE to `git config --global` (so it does NOT use protocol.Apply) but owns its preview+confirm
// via the shared ConfirmFunc. aliasName is the resolved name (default "stagehand").
type gitAliasEntry struct {
	git       git.Git   // repo-independent for --global; cwd from os.Getwd() (no-op for global scope)
	aliasName string    // resolved (never "" — defaultEntries resolves "" → "stagehand")
}

// newGitAliasEntry builds the entry for the current invocation (reads the resolved --alias-name).
func newGitAliasEntry() *gitAliasEntry {
	name := flagAliasName
	if name == "" {
		name = defaultAliasName
	}
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "." // global config ignores cwd; never fatal
	}
	return &gitAliasEntry{git: git.New(cwd), aliasName: name}
}

func (e *gitAliasEntry) Name() string { return gitAliasTarget }

// aliasKey returns "alias.<name>".
func (e *gitAliasEntry) aliasKey() string { return "alias." + e.aliasName }

// isOurs reports whether a stored alias value (incl. its leading `!`) is stagehand's command.
func isOurs(storedValue string) bool { return strings.TrimPrefix(storedValue, "!") == defaultAliasName }

// Detect — FR-I2: git-alias needs only git. exec.LookPath("git"); nil if present.
func (e *gitAliasEntry) Detect(context.Context) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found on PATH: %w", err)
	}
	return nil
}

// ConfigPath — the global gitconfig path (list CONFIG column; display-only, best-effort).
func (e *gitAliasEntry) ConfigPath(context.Context) (string, error) {
	if g := os.Getenv("GIT_CONFIG_GLOBAL"); g != "" {
		if abs, err := filepath.Abs(g); err == nil {
			return abs, nil
		}
		return g, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve global gitconfig: %w", err)
	}
	return filepath.Join(home, ".gitconfig"), nil
}

// Status — read back alias.<name>: unset → NotInstalled; ours → Installed; foreign → Foreign.
func (e *gitAliasEntry) Status(ctx context.Context) (integrate.Status, error) {
	v, found, err := e.git.ConfigGlobalGet(ctx, e.aliasKey())
	if err != nil {
		return integrate.StatusNotInstalled, err
	}
	if !found {
		return integrate.StatusNotInstalled, nil
	}
	if isOurs(v) {
		return integrate.StatusInstalled, nil
	}
	return integrate.StatusForeign, nil
}

// Install — FR-I4: show command+usage (+ conflict if foreign), confirm, then `git config --global alias.<name> '!stagehand'`.
func (e *gitAliasEntry) Install(ctx context.Context, opts integrate.InstallOptions) (integrate.InstallResult, error) {
	out := opts.Out
	if out == nil {
		out = os.Stderr
	}
	res := integrate.InstallResult{Outcome: integrate.OutcomeNoChange, Target: e.Name(), Path: e.configPathOr(""), Backup: ""}

	cur, found, err := e.git.ConfigGlobalGet(ctx, e.aliasKey())
	if err != nil {
		return res, fmt.Errorf("read alias %s: %w", e.aliasName, err)
	}

	// Already ours ⇒ idempotent NoChange (do NOT rewrite).
	if found && isOurs(cur) {
		return res, nil
	}

	// Build the preview (command + usage + conflict warning if foreign).
	preview := fmt.Sprintf("Command:  git config --global %s '%s'\nResult:   git %s  →  stagehand\n",
		e.aliasKey(), stagehandAliasValue, e.aliasName)
	if found { // foreign (not ours) — surface before overwriting (FR-I4)
		preview += fmt.Sprintf("\nWARNING: %s is currently set to %q (not stagehand) — it will be overwritten.\n",
			e.aliasKey(), cur)
	}

	if !opts.Yes {
		confirm := opts.Confirm
		if confirm == nil {
			confirm = integrate.DefaultConfirm // TTY-gated y/N; non-TTY auto-decline
		}
		if !confirm(out, res.Path, preview) {
			res.Outcome = integrate.OutcomeDeclined
			return res, nil // NOTHING written
		}
	}

	if err := e.git.ConfigGlobalSet(ctx, e.aliasKey(), stagehandAliasValue); err != nil {
		return res, fmt.Errorf("set alias %s: %w", e.aliasName, err)
	}
	if found {
		res.Outcome = integrate.OutcomeUpdated // overwrote a foreign alias
	} else {
		res.Outcome = integrate.OutcomeCreated // newly installed
	}
	return res, nil
}

// Remove — FR-I6: `git config --global --unset alias.<name>` ONLY when the value is ours. A foreign alias
// is left untouched (NoChange + note). An unset alias is NoChange.
func (e *gitAliasEntry) Remove(ctx context.Context, opts integrate.RemoveOptions) (integrate.RemoveResult, error) {
	out := opts.Out
	if out == nil {
		out = os.Stderr
	}
	res := integrate.RemoveResult{Outcome: integrate.OutcomeNoChange, Target: e.Name(), Path: e.configPathOr(""), Backup: ""}

	cur, found, err := e.git.ConfigGlobalGet(ctx, e.aliasKey())
	if err != nil {
		return res, fmt.Errorf("read alias %s: %w", e.aliasName, err)
	}
	if !found {
		return res, nil // nothing to remove — NoChange
	}
	if !isOurs(cur) {
		// FR-I6: NEVER remove a foreign alias. Inform + NoChange.
		fmt.Fprintf(out, "stagehand: %s is set to %q (not stagehand); leaving it unchanged.\n", e.aliasKey(), cur)
		return res, nil
	}

	// Ours — preview + confirm the unset (FR-I3c), then unset.
	preview := fmt.Sprintf("Command:  git config --global --unset %s\nResult:   removes `git %s`\n",
		e.aliasKey(), e.aliasName)
	if !opts.Yes {
		confirm := opts.Confirm
		if confirm == nil {
			confirm = integrate.DefaultConfirm
		}
		if !confirm(out, res.Path, preview) {
			res.Outcome = integrate.OutcomeDeclined
			return res, nil
		}
	}
	if _, err := e.git.ConfigGlobalUnset(ctx, e.aliasKey()); err != nil {
		return res, fmt.Errorf("unset alias %s: %w", e.aliasName, err)
	}
	res.Outcome = integrate.OutcomeRemoved
	return res, nil
}

// configPathOr returns ConfigPath or "" on error (for Result.Path; never fatal).
func (e *gitAliasEntry) configPathOr(fallback string) string {
	if p, err := e.ConfigPath(context.Background()); err == nil && p != "" {
		return p
	}
	return fallback
}
```

```go
// === internal/cmd/integrate.go — S2's file, the ONE edit (defaultEntries) ===
// BEFORE (S2):
//   var defaultEntries = func() []integrate.Entry { return nil }
// AFTER (T2.S1):
var defaultEntries = func() []integrate.Entry {
	return []integrate.Entry{ newGitAliasEntry() } // T2.S2 appends &lazygitEntry{...} here later
}

// === internal/cmd/integrate_test.go — S2's file, the ONE edit (resetIntegrateFlags) ===
// T2.S1 APPENDS (inside the existing resetIntegrateFlags, after the --yes reset):
//   flagAliasName = ""
//   if f := integrateInstallCmd.Flags().Lookup("alias-name"); f != nil && f.Changed { f.Changed = false }
//   if f := integrateRemoveCmd.Flags().Lookup("alias-name"); f != nil && f.Changed { f.Changed = false }
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD internal/git/git.go — ConfigGlobalGet/Set/Unset interface methods + impls
  - ADD to the Git INTERFACE (top of git.go, after HooksPath): ConfigGlobalGet(ctx,key)(value,found,err),
    ConfigGlobalSet(ctx,key,value)error, ConfigGlobalUnset(ctx,key)(found,err). Full doc comments (see Blueprint).
  - ADD impls on *gitRunner via g.run(): each runs `config --global [--get|--unset] <key>` (Set: `config
    --global <key> <value>`). Branch on code: Get → 0 found(trimmed)/1 missing(nil)/else error; Set →
    0 ok/else error; Unset → 0 removed(found=true)/5 missing(found=false)/else error. Trim stderr; fmt.Errorf %w.
  - FOLLOW pattern: internal/git/git.go HooksPath impl + internal/config/git.go gitConfigGet exit-code branching.
  - GOTCHA: run() inherits parent env → tests isolate via t.Setenv("GIT_CONFIG_GLOBAL",…). -C workDir is a
    no-op for --global. value passed as a SINGLE argv element (not sh -c) → "!stagehand" stored verbatim.
  - NAMING: ConfigGlobalGet/Set/Unset (Global = the user-global gitconfig scope; distinct from repo-local).

Task 2: CREATE internal/git/configglobal_test.go — the methods' tests (hookspath_test.go style)
  - TESTS (construct g := New(t.TempDir()); isolate via t.Setenv("GIT_CONFIG_GLOBAL", tmpfile)):
    * TestConfigGlobalGet_FoundAndMissing: set alias.x '!stagehand' via g.ConfigGlobalSet → Get returns
      ("!stagehand", true, nil); Get a missing key → ("", false, nil).
    * TestConfigGlobalSet_WritesValue: Set alias.y '!stagehand' → read back via Get == "!stagehand" (the `!`
      is preserved — proves single-argv, not sh -c).
    * TestConfigGlobalUnset_PresentAndMissing: Set then Unset → (true, nil); Get now missing; Unset again →
      (false, nil) (exit 5 ⇒ not an error).
    * TestConfigGlobal_Isolation: write to a tmp GIT_CONFIG_GLOBAL file; assert the file gains the entry;
      (optional) confirm the REAL ~/.gitconfig is untouched by checking a sentinel key before/after.
  - GOTCHA: do NOT use minGitEnv (it strips GIT_CONFIG_GLOBAL); rely on inherited env via t.Setenv. No repo
    init needed. Follow the hookspath_test.go file/func style (t.TempDir + New + assert).
  - PLACEMENT: internal/git/configglobal_test.go (package git, in-package like the other *_test.go).

Task 3: CREATE internal/cmd/integrate_gitalias.go — gitAliasEntry + --alias-name + init()
  - IMPLEMENT gitAliasEntry (all 6 Entry methods) per the Blueprint's data-models block: Name/Detect/
    ConfigPath/Status/Install/Remove + helpers (aliasKey, isOurs, configPathOr, newGitAliasEntry).
  - IMPLEMENT `var flagAliasName string` + init() registering --alias-name on integrateInstallCmd AND
    integrateRemoveCmd (StringVar, default "").
  - GOTCHA: Install returns Created/Updated/NoChange/Declined per the mapping; foreign-install surfaces the
    conflict in the preview then overwrites after confirm. Remove returns Removed/NoChange/Declined;
    foreign-remove is NoChange + note (NEVER unset). Backup always "". Reuse integrate.DefaultConfirm when
    opts.Confirm==nil; skip confirm when opts.Yes.
  - DEPENDENCIES: internal/git (Git + the 3 methods), internal/integrate (Entry/Status/Options/Result/Outcome/
    ConfirmFunc/DefaultConfirm). Imports: stdlib only beyond those.
  - PLACEMENT: internal/cmd/integrate_gitalias.go (keeps git-alias logic isolated from S2's integrate.go).

Task 4: EDIT internal/cmd/integrate.go — defaultEntries seam (S2's file)
  - CHANGE `var defaultEntries = func() []integrate.Entry { return nil }` to return
    `[]integrate.Entry{ newGitAliasEntry() }`. Add a comment that T2.S2 appends &lazygitEntry later.
  - GOTCHA: newGitAliasEntry() reads flagAliasName (populated — defaultEntries is called per-command after
    flag parse) + os.Getwd() (global config ignores cwd). Do NOT touch anything else in integrate.go.

Task 5: EDIT internal/cmd/integrate_test.go — resetIntegrateFlags (S2's file)
  - APPEND to the existing resetIntegrateFlags (after S2's --yes reset): `flagAliasName = ""` + reset the
    Changed bit on BOTH integrateInstallCmd and integrateRemoveCmd's "alias-name" flag (see Blueprint).
  - GOTCHA: resetIntegrateFlags is S2's helper; add the minimum lines. This is the only edit to S2's test file.

Task 6: CREATE internal/cmd/integrate_gitalias_test.go — the matrix + wiring
  - HELPER: newIsolatedGitAliasEntry(t, name) builds &gitAliasEntry{git: git.New(t.TempDir()), aliasName: name}
    + t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig")) (full isolation). Returns the
    entry + a fixed-bool Confirm (for the confirm-flow tests).
  - ENTRY-LEVEL TESTS (call methods directly; real git via the isolated Git instance):
    * TestGitAlias_Status_States: unset → NotInstalled; install (Yes) → Installed; foreign (set alias.x to
      '!other' via ConfigGlobalSet) → Foreign; ours (set '!stagehand') → Installed.
    * TestGitAlias_Install_Creates: unset → Install(Yes) → OutcomeCreated; read-back alias.name == "!stagehand".
    * TestGitAlias_Install_IdempotentAlreadyOurs: pre-set '!stagehand' → Install(Yes) → OutcomeNoChange; no
      write (value unchanged).
    * TestGitAlias_Install_ForeignOverwritesAfterConfirm: pre-set '!other' → Install(Yes) → OutcomeUpdated;
      read-back == "!stagehand" (overwrote); preview string CONTAINS the conflict warning + the current value.
    * TestGitAlias_Install_DeclineWritesNothing: Confirm stub returns false → OutcomeDeclined; alias UNCHANGED
      (still '!other' or unset).
    * TestGitAlias_Install_ConfirmReceivesPreview: capture the `diff` arg the Confirm stub receives → contains
      "git config --global alias.X '!stagehand'" + "git X" + (foreign) the overwrite warning.
    * TestGitAlias_Remove_Ours: install then Remove(Yes) → OutcomeRemoved; read-back missing.
    * TestGitAlias_Remove_ForeignRefuses: pre-set '!other' → Remove(Yes) → OutcomeNoChange; alias UNCHANGED
      (still '!other'); a note was written to opts.Out.
    * TestGitAlias_Remove_UnsetIsNoOp: unset → Remove(Yes) → OutcomeNoChange.
    * TestGitAlias_Remove_Decline: ours + Confirm false → OutcomeDeclined; alias unchanged.
    * TestGitAlias_Detect: present → nil; t.Setenv("PATH","") → non-nil error containing "git not found".
    * TestGitAlias_ConfigPath: GIT_CONFIG_GLOBAL set → returns it (absolute); unset (t.Setenv to "") +
      HOME set → returns $HOME/.gitconfig.
    * TestGitAlias_CustomAliasName: entry with aliasName "ci" → manages alias.ci (install → `git ci`); the
      preview mentions `git ci`.
  - WIRING TESTS (saveRootState/restoreRootState + resetIntegrateFlags + SetArgs + Execute, defaultEntries
    swapped to newGitAliasEntry with an isolated GIT_CONFIG_GLOBAL):
    * TestIntegrateInstall_GitAlias_Execute: `integrate install git-alias --yes` → exit 0; stdout status line;
      alias.stagehand == "!stagehand" in the isolated global config.
    * TestIntegrateAliasNameFlag: `integrate install git-alias --yes --alias-name ci` → alias.ci set (not
      alias.stagehand).
    * TestIntegrateRemove_GitAlias_Execute: install then `integrate remove git-alias --yes` → exit 0; alias gone.
  - FOLLOW pattern: internal/git/hookspath_test.go (t.TempDir + assert style); providers_test.go
    (saveRootState/SetArgs/Execute + substring asserts); resetIntegrateFlags between Execute tests.
  - GOTCHA: every test that touches the global config MUST t.Setenv("GIT_CONFIG_GLOBAL", tmpfile) FIRST (isolation
    is non-negotiable — the real ~/.gitconfig must never be touched). Reset flagAliasName via resetIntegrateFlags.

Task 7: EDIT docs/cli.md — git-alias target section (Mode A, EXTENDS S2's integrate group section)
  - ADD a `### \`git-alias\` target` subsection within/after S2's `integrate` group section: what it does
    (registers `git stagehand` via `git config --global alias.stagehand '!stagehand'` — git itself writes the
    .gitconfig); the `--alias-name <n>` override; the conflicting-alias behavior (install surfaces a foreign
    `alias.<name>` before overwriting after confirm; remove never removes a foreign alias — FR-I6); what
    `integrate list` shows (DETECTED ✓ — needs only git; STATUS not-installed/installed/foreign; CONFIG =
    global gitconfig path). One example each for install/remove + `--alias-name`.
  - GOTCHA: S2 wrote the `integrate list/install/remove` GROUP subsections (Mode A); T2.S1 ADDS the
    git-alias TARGET detail + the --alias-name flag. Do NOT rewrite S2's group section — extend it.
  - FOLLOW the existing per-target heading + example-block + flag-table style (see the `hook` subsections).

Task 8: VERIFY build/test/lint (no go.mod change)
  - go build ./... ; go test ./internal/git/... ./internal/cmd/... -v ; go test ./... ;
    go vet ./... ; golangci-lint run ; gofmt -l internal/git internal/cmd. Confirm go.mod UNCHANGED
    (git diff go.mod empty — yaml.v3 is T2.S2's, NOT this subtask's).
```

### Implementation Patterns & Key Details

```go
// isOurs — the "is this alias stagehand's" test (external_deps.md §7). The stored value INCLUDES the `!`,
// so strip it and compare the COMMAND part (always "stagehand"); the alias NAME may be overridden.
func isOurs(storedValue string) bool { return strings.TrimPrefix(storedValue, "!") == defaultAliasName }

// The ConfigGlobal* impls branch on run()'s exit code (NOT stdout emptiness), mirroring config/git.go:
func (g *gitRunner) ConfigGlobalGet(ctx context.Context, key string) (string, bool, error) {
	stdout, stderr, code, err := g.run(ctx, g.workDir, "config", "--global", "--get", key)
	if err != nil {
		return "", false, err // git binary missing / context cancel / start failure (code == -1)
	}
	switch code {
	case 0:
		return strings.TrimSpace(stdout), true, nil
	case 1:
		return "", false, nil // missing key — NOT an error
	default:
		return "", false, fmt.Errorf("git config --global --get %s: exit %d: %s", key, code, strings.TrimSpace(stderr))
	}
}
// ConfigGlobalUnset is identical but exit 5 ⇒ found=false (git's "key not set" code); exit 0 ⇒ found=true.

// The isolation helper every gitAliasEntry test uses — non-negotiable (the real global config is never touched):
func newIsolatedGitAliasEntry(t *testing.T, name string) *gitAliasEntry {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	t.Setenv("GIT_CONFIG_GLOBAL", cfg) // REPLACES ~/.gitconfig → full isolation (run() inherits parent env)
	nm := name
	if nm == "" { nm = defaultAliasName }
	return &gitAliasEntry{git: git.New(t.TempDir()), aliasName: nm} // no repo init needed (--global)
}

// resetIntegrateFlags extension (appended to S2's helper, in integrate_test.go):
func resetIntegrateFlags(t *testing.T) { // S2's existing body resets flagIntegrateYes + --yes Changed
	t.Helper()
	flagIntegrateYes = false
	if f := integrateCmd.PersistentFlags().Lookup("yes"); f != nil && f.Changed { f.Changed = false }
	// --- T2.S1 additions: ---
	flagAliasName = ""
	for _, c := range []*cobra.Command{integrateInstallCmd, integrateRemoveCmd} {
		if f := c.Flags().Lookup("alias-name"); f != nil && f.Changed { f.Changed = false }
	}
}
```

### Integration Points

```yaml
PROVIDES (to the integrate command surface — already wired by S2):
  - gitAliasEntry                 # the first concrete integrate.Entry (Name="git-alias")
  - Git.ConfigGlobalGet/Set/Unset # repo-independent global-config access (reusable beyond git-alias)
  - defaultEntries now non-empty  # `integrate list` shows the git-alias row; install/remove drive it
  - --alias-name flag             # local on integrate install + remove

CONSUMES (do NOT re-implement):
  - integrate.Entry / Registry / Status / InstallOptions / InstallResult / RemoveOptions / RemoveResult (S2)
  - integrate.Outcome / ConfirmFunc / DefaultConfirm (S1)
  - the Git interface + run() exec seam (internal/git)
  - S2's defaultEntries seam + dispatchInstall/dispatchRemove/printIntegrateList + resetIntegrateFlags +
    the cobra integrate group (integrateCmd/integrateInstallCmd/integrateRemoveCmd).

OUT OF SCOPE (owned by sibling subtasks — do NOT implement):
  - S1 (done): the no-mangle protocol engine (protocol.Apply) — git-alias does NOT use it (FR-I4).
  - S2 (parallel/done): the command surface, registry, detection gating, dispatch, the integrate group.
  - T2.S2: the lazygit target (comment-preserving YAML via yaml.v3) — adds yaml.v3 to go.mod; appends
           &lazygitEntry to defaultEntries (co-resident — both append to the same seam).
  - P1.M7: README coherence (this subtask edits ONLY docs/cli.md).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagehand-competitor-feature-parity
gofmt -w internal/git/git.go internal/git/configglobal_test.go \
  internal/cmd/integrate_gitalias.go internal/cmd/integrate_gitalias_test.go
go build ./...            # the 3 new Git methods + gitAliasEntry compile against internal/integrate + internal/git
go vet ./...
golangci-lint run
# go.mod MUST be unchanged (no new require — yaml.v3 is T2.S2's, not this subtask's):
git diff --name-only go.mod && echo "WARN: go.mod touched" || echo "go.mod clean OK"
# Expected: zero errors; go.mod clean.
```

### Level 2: Unit Tests (Component Validation)

```bash
go test ./internal/git/... -run ConfigGlobal -v
# Expected: TestConfigGlobalGet_FoundAndMissing, TestConfigGlobalSet_WritesValue,
#   TestConfigGlobalUnset_PresentAndMissing, TestConfigGlobal_Isolation — all pass; the `!` is preserved.

go test ./internal/cmd/... -run GitAlias -v
# Expected: Status_States, Install_Creates, Install_IdempotentAlreadyOurs, Install_ForeignOverwritesAfterConfirm,
#   Install_DeclineWritesNothing, Install_ConfirmReceivesPreview, Remove_Ours, Remove_ForeignRefuses,
#   Remove_UnsetIsNoOp, Remove_Decline, Detect, ConfigPath, CustomAliasName — all pass.

go test ./internal/cmd/... -run Integrate -v   # S2's dispatch tests + T2.S1's Execute wiring
# Expected: S2's matrix still green + TestIntegrateInstall_GitAlias_Execute, TestIntegrateAliasNameFlag,
#   TestIntegrateRemove_GitAlias_Execute pass.

go test ./...     # full suite — confirm no regression (config/providers/hook/git)
# Expected: all pass; the REAL global config is never touched (isolation via GIT_CONFIG_GLOBAL).
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagehand ./cmd/stagehand

# list now shows the git-alias row (DETECTED ✓, STATUS not installed, CONFIG = global gitconfig path)
/tmp/stagehand integrate list

# Real install against a THROWAWAY global config (never your real ~/.gitconfig in this manual check):
export GIT_CONFIG_GLOBAL=/tmp/stagehand-test.gitconfig
rm -f "$GIT_CONFIG_GLOBAL"
/tmp/stagehand integrate install git-alias --yes
git config --global --get alias.stagehand        # → "!stagehand"  (the `!` proves single-argv)
GIT_CONFIG_GLOBAL=/dev/null git config --global --get alias.stagehand 2>&1 | grep -q . && \
  echo "ERROR: real global config was touched" || echo "OK: real ~/.gitconfig untouched"
# Expected: alias.stagehand == "!stagehand" in /tmp/stagehand-test.gitconfig ONLY.

# conflicting-alias behavior: seed a foreign value, install surfaces it then overwrites
echo '[alias]' > "$GIT_CONFIG_GLOBAL"; git config --global alias.stagehand '!my-thing'
/tmp/stagehand integrate install git-alias       # (no --yes) → preview shows the conflict, asks y/N
# Type y → overwrites; type N → Declined (alias unchanged).
git config --global --get alias.stagehand        # after y → "!stagehand"

# remove refuses a foreign alias (FR-I6)
git config --global alias.stagehand '!someone-elses'
/tmp/stagehand integrate remove git-alias --yes  # → NoChange + "leaving it unchanged"; alias NOT removed
git config --global --get alias.stagehand        # still "!someone-elses"

# --alias-name override
/tmp/stagehand integrate install git-alias --yes --alias-name ci
git config --global --get alias.ci               # → "!stagehand"  (alias.ci, usage `git ci`)

# works OUTSIDE a git repo (integrate skips config.Load)
cd /tmp && /tmp/stagehand integrate install git-alias --yes; echo "exit=$?"
# Expected: exit 0 (global config doesn't need a repo).
unset GIT_CONFIG_GLOBAL
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Idempotency stress (the "run it 10 times" check) — re-install is a NoChange, no duplicate, no drift:
export GIT_CONFIG_GLOBAL=/tmp/sh-idem.gitconfig; rm -f "$GIT_CONFIG_GLOBAL"
for i in $(seq 1 10); do /tmp/stagehand integrate install git-alias --yes >/dev/null; done
test "$(git config --global --get alias.stagehand)" = "!stagehand" && echo "OK: stable across 10 installs"
# Expected: exactly one alias.stagehand == "!stagehand" after 10 runs (NoChange after the first).
unset GIT_CONFIG_GLOBAL

# Never-clobber guarantee under foreign-on-remove (FR-I6) — re-run with -count=1 to defeat the cache:
go test ./internal/cmd/... -run 'GitAlias_Remove_ForeignRefuses' -v -count=1
# Isolation audit — confirm no test wrote the real global config (run the suite, then check a sentinel):
SENTINEL_BEFORE="$(git config --global --get stagehand.t2s1.isolation-audit 2>/dev/null)"
go test ./internal/cmd/... -run GitAlias -count=1
SENTINEL_AFTER="$(git config --global --get stagehand.t2s1.isolation-audit 2>/dev/null)"
[ "$SENTINEL_BEFORE" = "$SENTINEL_AFTER" ] && echo "OK: real global config untouched by tests"
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...`; `go test ./...`; `go vet ./...`; `golangci-lint run`; `gofmt -l` clean.
- [ ] `go.mod` UNCHANGED (no new `require` — yaml.v3 is T2.S2's).
- [ ] `Git.ConfigGlobalGet/Set/Unset` added to the interface + `gitRunner` impls (run()-based); exit-code
      semantics: `--get` exit 1 = not-found (found=false), `--unset` exit 5 = not-set (found=false), else error.
- [ ] Tests isolate via `t.Setenv("GIT_CONFIG_GLOBAL", <tmpfile>)` — the real global config is NEVER touched.

### Feature Validation
- [ ] `integrate list` shows `git-alias` (DETECTED ✓, STATUS not-installed/installed/foreign, CONFIG = global gitconfig path).
- [ ] `integrate install git-alias` previews `git config --global alias.stagehand '!stagehand'` + `git stagehand`,
      confirms (y/N; `--yes` skips), then sets; re-run is NoChange (idempotent).
- [ ] A foreign `alias.<name>` is surfaced in the preview before overwrite (install → Updated after confirm).
- [ ] `integrate remove git-alias` unsets ONLY when the value is ours (Removed); a foreign alias is refused
      (NoChange + note; FR-I6); an unset alias is NoChange.
- [ ] `--alias-name <n>` overrides the name on install AND remove.
- [ ] `git stagehand` (or `git <name>`) runs stagehand after install.
- [ ] Decline (user N / non-TTY no --yes) writes nothing; Outcome=Declined.
- [ ] Works outside a git repo (integrate skips config.Load; global config needs no repo).

### Code Quality Validation
- [ ] `gitAliasEntry` implements all six `integrate.Entry` methods; reuses S1's `Outcome`/`ConfirmFunc`/
      `DefaultConfirm` (does NOT re-invent them; does NOT call `protocol.Apply`).
- [ ] `InstallResult.Backup` is always `""` (git owns the file); Outcome per the action×state mapping.
- [ ] The only edits to S2's files are `defaultEntries` (return the entry) + `resetIntegrateFlags`
      (reset `--alias-name`) — minimal, additive.
- [ ] git-alias logic is isolated in `integrate_gitalias.go`/`_test.go` (low merge-friction with S2 + T2.S2).
- [ ] Follows existing conventions: per-method Git test file (hookspath_test.go), local-flag-on-install
      (hook.go --strict), exitcode routing (providers.go), t.TempDir + assert tests.

### Documentation & Deployment
- [ ] docs/cli.md has the `git-alias` target subsection (Mode A) extending S2's `integrate` group section:
      `--alias-name`, conflicting-alias behavior (install surfaces/overwrites; remove refuses foreign), `git
      config` delegation, what `list` shows.
- [ ] No README.md edit (P1.M7 owns the coherence sweep).

---

## Anti-Patterns to Avoid

- ❌ Don't use `protocol.Apply` for git-alias — FR-I4 delegates the `.gitconfig` edit to `git config`; git owns the file. git-alias has its OWN preview+confirm (the preview is a command+usage string, not a unified diff; Backup is always "").
- ❌ Don't compare the raw stored value (with `!`) to `"stagehand"` — strip the leading `!` first (external_deps.md §7): `TrimPrefix(value,"!") == "stagehand"`. The `!` is part of the stored value.
- ❌ Don't treat `--get` exit 1 or `--unset` exit 5 as errors — those are the "key not present" signals (found=false, nil err). Branch on the exit code, never on stdout emptiness.
- ❌ Don't write a NEW self-contained exec helper for git config — add `ConfigGlobalGet/Set/Unset` to the existing `Git` interface (delta_prd "existing git exec wrapper"; HooksPath precedent). config/git.go's separate gitExec exists ONLY to avoid an import cycle that integrate does not have.
- ❌ Don't use `minGitEnv` for the ConfigGlobal* tests — it strips `GIT_CONFIG_GLOBAL`. Rely on inherited env via `t.Setenv` (run() doesn't set cmd.Env). Every test that touches the global config MUST `t.Setenv("GIT_CONFIG_GLOBAL", <tmpfile>)` first — the real ~/.gitconfig must never be touched.
- ❌ Don't make foreign-on-remove an error or silently remove it — FR-I6: only unset when ours. A foreign alias is NoChange + a note (left untouched). (Contrast foreign-on-INSTALL, which DOES overwrite after surfacing + confirm.)
- ❌ Don't forget `--alias-name` on BOTH install and remove — you remove the alias by name (hook's `--strict` is install-only; git-alias differs).
- ❌ Don't construct gitAliasEntry at package-init time (flags aren't parsed yet) — `defaultEntries()` builds it per-command via `newGitAliasEntry()` (flagAliasName is populated then).
- ❌ Don't call `os.Exit`/exitcode from gitAliasEntry — it's the Entry impl returning plain errors + Outcomes; S2's dispatch wraps errors via exitcode.New. Decline/NoChange are NOT errors (exit 0).
- ❌ Don't add yaml.v3 / go-toml / any dependency — git-alias needs only stdlib + internal/git + internal/integrate. yaml.v3 is T2.S2's. go.mod stays unchanged.
- ❌ Don't rewrite S2's integrate.go/integrate_test.go — the only edits are `defaultEntries` (return the entry) + one `resetIntegrateFlags` block. Everything else git-alias owns lives in its own `integrate_gitalias.go`/`_test.go`.
- ❌ Don't write Execute-level tests for the Entry logic — call gitAliasEntry methods directly (real git + GIT_CONFIG_GLOBAL isolation); reserve Execute tests for `--alias-name` parsing/wiring only.

---

## Confidence Score

**9/10** — one-pass success likelihood is high. The contract is precisely specified: the EXACT three
`ConfigGlobal*` signatures + verified git exit-code semantics (`--get` exit 1, `--unset` exit 5 —
external_deps.md §7), the EXACT "ours" test (strip `!`, compare to `stagehand`), the EXACT Entry method set +
Options/Result shapes (S2's PRP) + Outcome/ConfirmFunc/DefaultConfirm (S1's PRP), the env-passthrough
isolation fact (run() inherits parent env ⇒ `t.Setenv("GIT_CONFIG_GLOBAL",…)` fully isolates), the exact
Outcome mapping per (action × current-state), the `defaultEntries`/`resetIntegrateFlags` single-line edits to
S2's files, the `--alias-name` flag placement (local on install AND remove — hook.go `--strict` precedent),
the preview+confirm reuse of `ConfirmFunc`, and the docs/cli.md placement (extends S2's integrate group
section). The residual uncertainty is cosmetic: the exact preview-string wording + the `list`/status-line
verbs (S2's `formatInstallResult`) — both assertable on stable substrings, so a wording tweak is a one-line
edit, not a redesign. No new dependency, no protocol.Apply, no config.Load, no repo dependency — the scope is
tight, and both the upstream contract (S1/S2) and the co-resident sibling (T2.S2) are named explicitly.
