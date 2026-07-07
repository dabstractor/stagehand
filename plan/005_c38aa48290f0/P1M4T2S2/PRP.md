name: "P1.M4.T2.S2 — lazygit integration target: comment-preserving customCommands upsert"
description: |
  Implement the SECOND concrete `integrate.Entry` target — **`lazygit`** (PRD §9.21 FR-I5, FR-I6, §15.3,
  §15.5). Adds ONE `customCommands` entry to lazygit's `config.yml` via a **comment-preserving, node-level
  YAML round-trip** (`gopkg.in/yaml.v3` Node API), driven through S1's no-mangle `integrate.Apply` protocol
  (parse-first refusal → idempotent marker upsert → unified-diff preview+confirm → timestamped backup →
  atomic write → re-parse validate+auto-restore → create-if-missing). Entry defaults (FR-I5): `key: '<c-a>'`
  (`--key <k>` override), `context: 'files'`, `command: 'stagecoach'`, `output: 'none'`, `loadingText:
  'Generating commit message…'`, `description: 'stagecoach: AI commit'`, marked with a `# stagecoach-integration`
  `LineComment` on the entry's `key` value scalar for FR-I3b idempotency (replace, never duplicate). Config
  discovery: `lazygit --print-config-dir` when available, else the platform default (`os.UserConfigDir()` +
  `lazygit/config.yml`). `integrate install|remove lazygit` end-to-end with uninstall symmetry (FR-I6: remove
  restores entry-absence — leaves `customCommands: []` if it was the only entry; lazygit tolerates empty).
  **Adds `gopkg.in/yaml.v3 v3.0.1` to go.mod (archived upstream — pinned + documented why).** Two adapters
  live in ONE new file: `lazygitTarget` (implements S1's `integrate.Target` — the yaml.v3 Node surgery) +
  `lazygitEntry` (implements S2's `integrate.Entry` — wires the target through `integrate.Apply`).
  **Consumer of S1 (Target/Apply/Outcome/ConfirmFunc/DefaultConfirm/BackupPath/Action) + S2
  (Entry/Status/InstallOptions/Result/Registry + defaultEntries seam + dispatch + resetIntegrateFlags) +
  co-resident with T2.S1 (git-alias — both append to defaultEntries/resetIntegrateFlags).** Adds the
  docs/cli.md lazygit target section (Mode A). NO README edit (P1.M7 owns that). The yaml.v3 Node API
  surgery (locate-via-pair-walk, marker-on-value-scalar, insert/replace/remove, SetIndent(2) encode,
  create-if-missing, corrupt-YAML refuse) is VERIFIED EMPIRICALLY in research/yaml-node-api.md.

---

## Goal

**Feature Goal**: Ship the `lazygit` integration target end-to-end so `stagecoach integrate install lazygit`
adds exactly one stagecoach `customCommands` entry to lazygit's `config.yml` — surgically, comment-preserving,
idempotent — and `stagecoach integrate remove lazygit` cleanly undoes it, both through S1's no-mangle protocol
(parse-first, preview+confirm, backup, validate+restore). A hand-maintained, commented lazygit config is
never mangled: only the stagecoach-marker entry is touched; all other content (other customCommands, GUI/git
blocks, user comments) is preserved. A corrupt `config.yml` is HARD-REFUSED (never written). A re-install of
an already-present stagecoach entry is `NoChange` (no write). The stagecoach entry is identified by its
`# stagecoach-integration` marker comment (replace, never duplicate) — independent of the `key` binding.

**Deliverable**:
1. `go.mod` + `go.sum` — `require gopkg.in/yaml.v3 v3.0.1` (pinned; archived-upstream note in a source comment).
2. `internal/cmd/integrate_lazygit.go` — `lazygitTarget` (yaml.v3 adapter: `integrate.Target`) +
   `lazygitEntry` (`integrate.Entry`: Name/Detect/ConfigPath/Status/Install/Remove) + `--key` flag (local on
   install AND remove, mirroring `--alias-name`) + `init()` + the entry-template constant + path resolver.
3. `internal/cmd/integrate_lazygit_test.go` — golden-file round-trip tests + full Entry/Apply matrix +
   `--key` wiring, all isolated via temp files (NEVER the real `~/.config/lazygit/config.yml`).
4. `internal/cmd/testdata/lazygit/golden_input.yml` (+ `golden_corrupt.yml`) — hand-maintained fixtures.
5. `internal/cmd/integrate.go` (S2's file) — ONE edit: `defaultEntries` appends `newLazygitEntry()`.
6. `internal/cmd/integrate_test.go` (S2's file) — ONE edit: `resetIntegrateFlags` appends the `--key` reset.
7. `docs/cli.md` — lazygit target subsection (Mode A): entry defaults, `--key`, config-file discovery order,
   no-mangle behavior, what `list`/`status` show.

**Success Definition**: `integrate list` shows `lazygit` DETECTED ✓ (when on $PATH), STATUS
not-installed/installed/foreign, CONFIG = resolved `config.yml` path. `integrate install lazygit --yes`
writes a comment-preserving entry (verified by re-parsing: marker present, all other entries/comments
preserved). Re-run → "No changes for lazygit (already installed)". `integrate remove lazygit --yes` deletes
ONLY the marker entry (other entries intact); re-run → "No changes". A `--key '<c-s>'` install writes that
binding. A corrupt `config.yml` → install refuses with a parse-error message and writes nothing (exit 1).
Tests never touch the real config. `go build ./...`, `go test ./...`, `go vet ./...`, `golangci-lint run`,
`gofmt -l` all green; `go.mod` gains exactly one `require` line (yaml.v3).

## User Persona

**Target User**: the "plan-holder" (PRD §7.1) who lives in lazygit and wants `stagecoach` one keystroke away
— press `<c-a>` in the files panel, get an AI commit, stay in the UI (US8: `output: 'none'`). They run
`stagecoach integrate install lazygit` once. Their fear: "did stagecoach clobber my hand-tuned lazygit
config?" The comment-preserving, parse-first, backup+restore protocol is the answer.

**Use Case**: `stagecoach integrate install lazygit` → preview the unified diff (entry added, nothing else) →
confirm `y` → the entry lands; pressing `<c-a>` in lazygit's files panel runs `stagecoach`. Later `integrate
remove lazygit` removes only the stagecoach entry.

**User Journey**: `integrate list` (see lazygit DETECTED ✓, not installed, CONFIG=~/.config/lazygit/config.yml)
→ `integrate install lazygit` (see diff → `y`) → use `<c-a>` → later `integrate remove lazygit` (confirm →
entry gone, config otherwise untouched). `--yes` for scripts; `--key '<c-s>'` to pick a different binding.

**Pain Points Addressed**: no hand-editing of YAML; a comment-preserving round-trip means hand-maintained
formatting/comments survive; a parse failure never writes; a backup exists if anything goes sideways; the
marker makes re-install idempotent and remove surgical.

## Why

- **PRD §9.21 FR-I5**: lazygit adds a `customCommands` entry to its config file (located via `lazygit
  --print-config-dir`, else platform default); defaults `key:'<c-a>'`/`context:'files'`/`command:'stagecoach'`/
  `output:'none'`/`loadingText`/`description`, marked `# stagecoach-integration`; comment-preserving YAML edit
  via a node-level round-trip (yaml.v3 Node API); schema field names verified against the current lazygit
  release (FR-D5 — recorded in external_deps.md §1, verified 2026-07-02 against lazygit v0.62.2).
- **PRD §9.21 FR-I3 (a)–(g)**: the no-mangle write protocol — lazygit is the FILE-EDITING target that
  exercises the FULL machinery (unlike git-alias, which delegates the write to `git config` and skips it).
- **PRD §9.21 FR-I6**: uninstall symmetry — remove restores the file to its pre-stagecoach state for that entry.
- **PRD §15.5**: the documented `customCommands` shape stagecoach writes.
- **architecture/external_deps.md §1–§2** (VERIFIED): `output` (not `subprocess`/`showOutput`), `loadingText`
  valid, files-panel context is `files`, `lazygit --print-config-dir` exists (no `--config-dir`), platform
  defaults; yaml.v3 v3.0.1 node upsert works with `SetIndent(2)` but whole-doc normalization is unavoidable
  (the protocol — not the library — is the no-mangle guarantee).
- **Scope fences**: CONSUMES S1's `integrate.Target`/`Apply`/`Outcome`/`ConfirmFunc`/`DefaultConfirm`/
  `Action`/`ApplyResult`/`BackupPath` and S2's `integrate.Entry`/`Status`/`InstallOptions`/`RemoveOptions`/
  `InstallResult`/`RemoveResult` + the `defaultEntries` seam + dispatch + `resetIntegrateFlags`; PROVIDES the
  lazygit target + the yaml.v3 direct dependency. Co-resident with T2.S1 (git-alias) — both append to the same
  `defaultEntries`/`resetIntegrateFlags` seams additively. Does NOT re-implement the protocol (S1) or the
  command surface (S2); does NOT touch README (P1.M7).

## What

Two co-located adapters in one new file (`internal/cmd/integrate_lazygit.go`):

1. **`lazygitTarget`** — implements S1's `integrate.Target` over `gopkg.in/yaml.v3`'s Node API. Holds the
   configured `key` + the parsed `*yaml.Node` root. `Parse` round-trips bytes→node (any error ⇒ refuse).
   `HasEntry` walks the `customCommands` sequence (LOCATED BY PAIR-WALKING the top mapping — never a hardcoded
   index) for an item whose `key` value scalar carries the `stagecoach-integration` `LineComment`. `Upsert`
   builds the entry node from a YAML template, REPLACES the marked item if present (else APPENDS), and
   re-encodes with `SetIndent(2)`. `Remove` deletes the marked item (leaves `customCommands: []` if emptied).
   `Validate` re-parses on a throwaway (clean, side-effect-free probe).
2. **`lazygitEntry`** — implements S2's `integrate.Entry`. `Install`/`Remove` construct a `lazygitTarget`,
   hand it to `integrate.Apply` (the FR-I3 envelope), and map `ApplyResult`→`InstallResult`/`RemoveResult`.
   `Detect` = `exec.LookPath("lazygit")`. `ConfigPath` = `lazygit --print-config-dir`/`config.yml` else the
   platform default. `Status` = marker-present→Installed / unmarked-item-with-our-key→Foreign / else→NotInstalled.

### Success Criteria

- [ ] `go.mod`: `require gopkg.in/yaml.v3 v3.0.1` added (and go.sum updated); a source comment notes it is
      archived upstream (2025) and pinned for the Node API (FR-D5).
- [ ] `internal/cmd/integrate_lazygit.go`: `lazygitTarget` implements all six `integrate.Target` methods
      (Marker/Parse/HasEntry/Upsert/Remove/Validate) via the yaml.v3 Node API; `lazygitEntry` implements all
      six `integrate.Entry` methods (Name/Detect/ConfigPath/Status/Install/Remove); `--key` flag (local on
      `integrateInstallCmd` AND `integrateRemoveCmd`, shared `flagLazygitKey`, default `""`→`"<c-a>"`);
      `init()` registers the flag; entry template + path resolver helpers.
- [ ] `internal/cmd/integrate_lazygit_test.go`: golden-file round-trip (only the marked node changes
      semantically — other entries/comments preserved), idempotent re-upsert (NoChange), marker
      replace-not-duplicate, remove restores entry-absence, remove preserves sibling entries, corrupt YAML →
      Apply refuses (writes nothing), create-if-missing (absent file → created with one entry; absent
      `customCommands` key → appended to existing top-level keys), `--key` custom binding, Status three states
      + Foreign, Detect PATH gate, ConfigPath discovery — all temp-file-isolated.
- [ ] `internal/cmd/integrate.go` (S2's): `defaultEntries` returns `[]integrate.Entry{ newGitAliasEntry(),
      newLazygitEntry() }` (T2.S1 lands git-alias first; T2.S2 appends lazygit).
- [ ] `internal/cmd/integrate_test.go` (S2's): `resetIntegrateFlags` resets `flagLazygitKey` + the Changed bit
      on `integrateInstallCmd` and `integrateRemoveCmd`'s `key` flag (appended after T2.S1's `--alias-name` block).
- [ ] `docs/cli.md`: lazygit target section — entry defaults, `--key`, config discovery order, no-mangle +
      backup behavior, Status tokens, one install/remove example.
- [ ] `go build ./...` + `go test ./...` + `go vet ./...` + `golangci-lint run` + `gofmt -l` clean;
      `go.mod` gains exactly the one yaml.v3 `require`.

## All Needed Context

### Context Completeness Check

_This PRP names the EXACT `integrate.Target` + `integrate.Entry` method sets (read from the completed S1/S2
source, not the PRPs), the EXACT yaml.v3 Node surgery (verified empirically — locate-by-pair-walk,
marker-on-value-scalar, insert/replace/remove, `SetIndent(2)`, create-if-missing, corrupt-refuse), the EXACT
entry defaults + the `# stagecoach-integration` marker placement, the EXACT config discovery
(`lazygit --print-config-dir` → platform default), the `defaultEntries`/`resetIntegrateFlags` additive edits
to S2's files (co-resident with T2.S1's `--alias-name`), the `--key` flag placement (mirrors `--alias-name`),
the `integrate.Apply` envelope contract (Apply owns parse/backup/atomic/validate/diff; the Target owns the
surgical node edit), the test-isolation discipline (temp files, never the real config), and the docs/cli.md
placement. An implementer with no prior codebase knowledge can build it from this document + codebase access._

### Documentation & References

```yaml
- docfile: plan/005_c38aa48290f0/P1M4T2S2/research/yaml-node-api.md
  why: THE verified yaml.v3 Node-API playbook for this subtask — locate customCommands by PAIR-WALKING (the
       hardcoded-index trap, reproduced+avoided), the marker on the value scalar (LineComment substring
       match), build-entry-from-template (`doc.Content[0].Content[0]`), insert-vs-replace, remove (slice
       trick) leaving `customCommands: []`, encode (`NewEncoder`+`SetIndent(2)`+`Encode(&doc)`+`Close()`),
       create-if-missing (empty-file + absent-key), corrupt-YAML→refuse (`Unmarshal` non-nil err), and the
       whole-doc-normalization gotcha (idempotency relies on Apply's byte-equality of stagecoach's OWN writes).
  section: all
  critical: |
    NEVER index customCommands by a hardcoded position — walk topMap.Content IN PAIRS (Content[i]=key,
    Content[i+1]=value). The throwaway test that hardcoded Content[1] mutated the wrong node (gui's value)
    and left the marker untouched. The marker is on item.Content[1] (the `key` VALUE scalar), matched by the
    SUBSTRING "stagecoach-integration" (yaml keeps the "# " prefix in LineComment). enc.Close() is REQUIRED.

- docfile: plan/005_c38aa48290f0/architecture/external_deps.md
  why: §1 (lazygit schema, VERIFIED 2026-07-02 v0.62.2) — current field is `output` (values none|terminal|
       log|logWithPty|popup), NOT the older subprocess/showOutput; `loadingText` is valid; files-panel context
       is `files`; `lazygit --print-config-dir` (short `-cd`) exists (no --config-dir); platform default paths.
       §2 (yaml.v3 comment-preserving, VERIFIED) — Node carries HeadComment/LineComment/FootComment; whole-doc
       re-encode may drop blank lines/normalize inline-comment spacing (the protocol is the guarantee); default
       encoder indent is 4 → SetIndent(2); go-yaml archived 2025.
  section: "## 1. lazygit customCommands schema (gates FR-I5); ## 2. Comment-preserving YAML in Go"
  critical: |
    Record in a source comment: "verified 2026-07-02 against lazygit v0.62.2" (FR-D5). Use `output: 'none'`
    (US8 — silent, stay in the UI). The marker comment is the idempotency identity (FR-I3b), NOT the key.

- docfile: plan/005_c38aa48290f0/P1M4T1S1/PRP.md   (and the COMPLETED source internal/integrate/protocol.go)
  why: THE upstream contract — `integrate.Target` interface (Marker/Parse/HasEntry/Upsert/Remove/Validate),
       `integrate.Apply(ctx, ApplyOptions{Path,Target,Action,Yes,Out,Confirm})(ApplyResult{Outcome,Path,Backup},error)`
       = the FR-I3 (a)–(g) envelope, `integrate.Outcome` (Created/Updated/Removed/Declined/NoChange),
       `integrate.Action` (ActionUpsert/ActionRemove), `integrate.ConfirmFunc`, `integrate.DefaultConfirm`,
       `integrate.ApplyResult`, `integrate.BackupPath`. lazygit's Target is driven by Apply; lazygitEntry.Install
       builds ApplyOptions and maps ApplyResult→InstallResult.
  section: "Data models; Implementation Tasks (PROVIDES)"
  critical: |
    Apply owns: read+parse-first (refuses on Parse error), idempotent (byte-equal ⇒ NoChange), preview-diff
    (git diff --no-index) + confirm, backup (only for existing-file modifications), MkdirAll parent AFTER
    confirm, atomic temp+rename write, re-parse Validate + auto-restore, success Outcome. The Target owns ONLY
    the surgical node edit. lazygitEntry.Install does NOT re-implement any of this — it calls Apply. Validate
    must NOT depend on prior Parse state (use a throwaway probe).

- docfile: plan/005_c38aa48290f0/P1M4T1S2/PRP.md   (and the COMPLETED source internal/integrate/registry.go)
  why: THE registry contract — `integrate.Entry` interface (Name/Detect/ConfigPath/Status/Install/Remove),
       `integrate.Status` (NotInstalled/Installed/Foreign + exact String tokens "not installed"/"installed"/
       "foreign"), `InstallOptions`/`RemoveOptions` {Yes,Out,Confirm}, `InstallResult`/`RemoveResult`
       {Outcome,Target,Path,Backup}, `integrate.Registry` (NewRegistry/Get/List sorted). lazygitEntry
       implements Entry; Outcome IS integrate.Outcome (copied from ApplyResult).
  section: "Data models; Implementation Tasks (PROVIDES)"
  critical: |
    Entry.Install decides HOW (lazygit → protocol.Apply; git-alias → git config). InstallOptions.Confirm==nil
    ⇒ DefaultConfirm. Decline/NoChange are NOT errors (exit 0). Status is INDEPENDENT of Detect (a target can
    be Installed with the tool since uninstalled). Target/Path/Backup are echoed in the result for the cmd
    status line (formatInstallResult/formatRemoveResult already exist in S2).

- docfile: plan/005_c38aa48290f0/P1M4T2S1/PRP.md
  why: THE sibling contract (implemented in parallel) — `gitAliasEntry` (the first concrete Entry) +
       `newGitAliasEntry()` factory + the `--alias-name` flag (local on install AND remove, shared
       `flagAliasName`) + its `resetIntegrateFlags` block + its `defaultEntries` append. lazygit MIRRORS this
       pattern (`--key` ↔ `--alias-name`, `newLazygitEntry()` ↔ `newGitAliasEntry()`). Co-residence: both
       append to defaultEntries (git-alias first, lazygit second) and both append to resetIntegrateFlags.
  section: "Implementation Tasks (Task 4 defaultEntries, Task 5 resetIntegrateFlags)"
  critical: |
    T2.S1 lands FIRST: after T2.S1, defaultEntries returns []integrate.Entry{ newGitAliasEntry() } and
    resetIntegrateFlags resets --alias-name. T2.S2 APPENDS lazygit to BOTH (additive, non-conflicting — each
    owns its own flag). Do NOT touch git-alias's lines. The resetIntegrateFlags block iterates
    []*cobra.Command{integrateInstallCmd, integrateRemoveCmd} — lazygit's --key mirrors it verbatim.

- docfile: plan/005_c38aa48290f0/prd_snapshot.md   (and PRD.md §9.21 / §15.3 / §15.5)
  why: §9.21 FR-I5 (lazygit entry + comment-preserving YAML), FR-I3 (the no-mangle protocol), FR-I6 (uninstall
       symmetry), FR-I2 (lazygit requires lazygit on $PATH), FR-I1 (list columns), §15.3 (subcommand reference),
       §15.4 (exit codes), §15.5 (the documented customCommands shape).
  section: "§9.21 (FR-I1/I2/I3/I5/I6), §15.3, §15.5"

- file: internal/integrate/protocol.go   (COMPLETED — S1)
  why: THE engine lazygit drives. (a) The `Target` INTERFACE — lazygitTarget implements Marker/Parse/HasEntry/
       Upsert/Remove/Validate. (b) `Apply` — the FR-I3 envelope (lines: read→Parse-refuse→HasEntry→
       Upsert/Remove→byte-equal-NoChange→previewDiff→confirm→backup→MkdirAll→atomicWrite→Validate→restore).
       (c) `DefaultConfirm` (TTY-gated y/N; non-TTY auto-decline; --yes bypassed by caller). (d) `BackupPath`.
  pattern: |
    Apply reads the file, calls Target.Parse(orig) (refuse on err), then Target.Upsert()/Remove(), compares
    bytes (NoChange if equal), preview-diff + confirm, backup, atomic write, Target.Validate(newBytes)
    (restore on err). lazygitEntry.Install: tgt:=&lazygitTarget{key:...}; r,err:=integrate.Apply(ctx,
    integrate.ApplyOptions{Path:cfgPath, Target:tgt, Action:integrate.ActionUpsert, Yes:opts.Yes, Out:opts.Out,
    Confirm:opts.Confirm}); map r.Outcome/r.Backup → InstallResult.
  gotcha: |
    Apply reads the file ITSELF and passes bytes to Parse — lazygitTarget does NOT read the file (it receives
    data). Apply creates parent dirs + writes atomically — lazygitTarget only emits bytes. Validate is called
    on a FRESH Apply with the just-written bytes; lazygitTarget.Validate must be a clean local probe (new
    yaml.Node, no reliance on prior Parse state).

- file: internal/integrate/registry.go   (COMPLETED — S2)
  why: THE Entry INTERFACE + Status + Options/Result shapes lazygitEntry implements + Registry. Also: S2's
       `formatInstallResult`/`formatRemoveResult` map Outcome→user verbs (Created→"Installed", Updated→"Updated
       … Backup:", Removed→"Removed …", Declined→"Declined", NoChange→"No changes (already installed)").
  pattern: "lazygitEntry.Install returns InstallResult{Outcome: r.Outcome, Target: lazygitTarget_name, Path: cfgPath, Backup: r.Backup}."
  gotcha: "Status.String() tokens are EXACT: 'not installed'/'installed'/'foreign' — match these in assertions."

- file: internal/cmd/integrate.go   (S2's — created by S2, git-alias appends first)
  why: THE file T2.S2 EDITS minimally: (1) `defaultEntries` (after T2.S1 it returns
       []integrate.Entry{ newGitAliasEntry() }) → append `newLazygitEntry()`; the seam is explicitly commented
       "T2.S2: append &lazygitEntry{...}". (2) T2.S2 APPENDS to `resetIntegrateFlags` (in integrate_test.go).
       S2 owns integrateCmd/integrateInstallCmd/integrateRemoveCmd + dispatchInstall/dispatchRemove/
       printIntegrateList/formatInstallResult — lazygit REFERENCES integrateInstallCmd/integrateRemoveCmd in
       its own init() (same package) to register --key. Do NOT rewrite S2's file — append/extend.
  pattern: "defaultEntries() builds each entry from its resolved flag + resolved config path, per-command."
  gotcha: "defaultEntries is called per-command after flag parse (runIntegrateList/Install/Remove) → flagLazygitKey
           is populated when newLazygitEntry() runs. Config-path resolution (lazygit --print-config-dir) is
           best-effort; on failure fall back to the platform default (never fatal)."

- file: internal/cmd/integrate_gitalias.go   (T2.S1's — the DIRECT template to mirror)
  why: THE pattern for a concrete Entry + a local flag + init() + the factory. Mirror its STRUCTURE for
       lazygit: package cmd; `var flagLazygitKey string`; init() registering `--key` on install AND remove;
       `newLazygitEntry()` factory; the entry struct with its six methods. (git-alias delegates the write to
       `git config`; lazygit delegates to `integrate.Apply` — the Install/Remove bodies differ, the SKELETON is
       identical.)
  pattern: "var flagX string; init(){ installCmd.Flags().StringVar(&flagX,...); removeCmd.Flags().StringVar(&flagX,...) }; newEntry() reads flagX, resolves path."
  gotcha: "git-alias's --alias-name is the ENTRY IDENTITY (alias.<name>); lazygit's --key is the BINDING stagecoach
           WRITES — the entry's identity is the MARKER comment. So lazygit Remove targets the marker (not the
           key). Register --key on both for UI symmetry + resetIntegrateFlags parity, but document remove-by-marker."

- file: internal/cmd/integrate_test.go   (S2's + T2.S1's block)
  why: THE resetIntegrateFlags location. It ALREADY contains (after T2.S1) the --alias-name reset block.
       T2.S2 APPENDS the --key reset after it. Also shows the Execute-wiring test convention:
       saveRootState/restoreRootState + resetIntegrateFlags + swap defaultEntries + rootCmd.SetArgs + Execute.
  pattern: |
    // append inside resetIntegrateFlags (after T2.S1's --alias-name block):
    flagLazygitKey = ""
    for _, c := range []*cobra.Command{integrateInstallCmd, integrateRemoveCmd} {
        if f := c.Flags().Lookup("key"); f != nil && f.Changed { f.Changed = false }
    }
  gotcha: "resetIntegrateFlags is shared — append only the --key block; do NOT modify the --yes or --alias-name lines."

- file: internal/cmd/hook.go
  why: REF — the local-flag-on-install precedent + exitcode routing + the "never clobber / restore" messaging
       style. lazygit's preview/confirm is owned by Apply (DefaultConfirm), NOT hand-rolled — contrast git-alias
       which hand-rolls its preview because it bypasses Apply.
  pattern: "cmd.Flags().StringVar(&flag, \"key\", default, \"...\") in init()."
  gotcha: "lazygit does NOT hand-roll a confirm — Apply does (DefaultConfirm writes the diff + asks y/N)."

- file: internal/cmd/config_test.go
  why: REF — the Execute-level test convention: `rootCmd.SetArgs([]string{...}); err := Execute(ctx)` +
       substring asserts (NOT saveRootState — that's in integrate_test.go). For lazygit Execute wiring use the
       saveRootState/restoreRootState/resetIntegrateFlags/swap-defaultEntries pattern from integrate_test.go.
  pattern: "rootCmd.SetArgs([]string{\"integrate\",\"install\",\"lazygit\",\"--yes\", \"--config-path\", tmp}); Execute(ctx)."
  gotcha: "Execute tests need an overrideable config path to avoid touching the real config — see --config-path below."

- file: go.mod / go.sum
  why: confirms deps are cobra/pflag/go-toml/v2 (NO yaml yet). This subtask ADDS `gopkg.in/yaml.v3 v3.0.1`.
       `go get gopkg.in/yaml.v3@v3.0.1` then tidy. NOTE go.sum already has a TRANSITIVE `go.yaml.in/yaml/v3
       v3.0.4` (via cobra) — that is a DIFFERENT module; the DIRECT dep is `gopkg.in/yaml.v3`.
  gotcha: "Pin v3.0.1 (the Node API is stable there; the module is archived so no newer release is coming).
           Add a source comment: '// gopkg.in/yaml.v3 v3.0.1 — archived upstream (2025); pinned for the Node
           API (HeadComment/LineComment/FootComment). Verified 2026-07-02.'"
```

### Current Codebase tree (relevant slice)

```bash
internal/
  integrate/                   # PACKAGE (S1+S2 — COMPLETE)
    protocol.go                # S1 — Target/Apply/Outcome/Action/ConfirmFunc/DefaultConfirm/ApplyResult (CONSUME)
    protocol_test.go           # S1 — blockTarget fake + the Apply matrix (REF test style) (do NOT edit)
    registry.go                # S2 — Entry/Status/InstallOptions/Result/Registry (CONSUME) (do NOT edit)
  git/
    git.go                     # REF — run() exec seam (lazygit does NOT use it; uses os/exec directly for --print-config-dir) (do NOT edit)
  cmd/
    integrate.go               # S2's — defaultEntries seam + dispatch + formatters + resetIntegrateFlags call site.
                               #   T2.S2 EDITS: defaultEntries appends newLazygitEntry().
    integrate_test.go          # S2's (+T2.S1's block) — T2.S2 EDITS: resetIntegrateFlags appends --key reset.
    integrate_gitalias.go      # T2.S1's — the DIRECT skeleton to mirror (do NOT edit) (REF)
    integrate_gitalias_test.go # T2.S1's — REF test style (do NOT edit)
    hook.go                    # REF — local-flag precedent (do NOT edit)
    config_test.go             # REF — Execute/SetArgs convention (do NOT edit)
    providers.go               # REF — exitcode/OutOrStdout conventions (do NOT edit)
  exitcode/exitcode.go         # REF (do NOT edit)
docs/
  cli.md                       # EDIT (Mode A) — lazygit target subsection (EXTENDS S2's integrate group section)
go.mod                         # EDIT — + gopkg.in/yaml.v3 v3.0.1
go.sum                         # EDIT — yaml.v3 hash (via go get/tidy)
```

### Desired Codebase tree with files to be added and responsibility of file

```bash
internal/cmd/
  integrate_lazygit.go         # NEW (T2.S2) — lazygitTarget (integrate.Target, yaml.v3 adapter) + lazygitEntry
                               #   (integrate.Entry) + --key flag (install+remove) + init() + entryTpl const +
                               #   resolveLazygitConfigPath() + newLazygitEntry() factory.
  integrate_lazygit_test.go    # NEW (T2.S2) — golden-file round-trip + Entry/Apply matrix + --key wiring.
  integrate.go                 # EDIT — defaultEntries appends newLazygitEntry() (after newGitAliasEntry()).
  integrate_test.go            # EDIT — resetIntegrateFlags appends the --key reset (after --alias-name block).
  testdata/lazygit/
    golden_input.yml           # NEW — hand-maintained commented lazygit config (round-trip fixture).
    golden_corrupt.yml         # NEW — malformed YAML (parse-refusal fixture).
docs/
  cli.md                       # EDIT — lazygit target subsection under S2's `integrate` group section.
go.mod                         # EDIT — require gopkg.in/yaml.v3 v3.0.1
go.sum                         # EDIT — (via go get/tidy)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (yaml.v3 is archived + whole-doc re-encodes): gopkg.in/yaml.v3 v3.0.1 is archived upstream (2025)
// — pin it + document why in a source comment (FR-D5). yaml.v3 re-encodes the ENTIRE document on Marshal, so
// byte-identity outside the edited node is NOT guaranteed (it may drop a blank line between sections or
// normalize inline-comment spacing — research/yaml-node-api.md §7). The no-mangle GUARANTEE therefore lives
// in integrate.Apply (preview-diff shows any normalization; re-parse validate + auto-restore catches
// breakage), NOT in the serializer. Idempotency works because after stagecoach's OWN write the file is in
// yaml.v3's canonical form, so a second Upsert is byte-identical ⇒ Apply NoChange.

// CRITICAL (NEVER hardcode the customCommands index): the top-level mapping's Content is a flat slice read
// IN PAIRS (Content[0]=key, Content[1]=value, …). customCommands is NOT at a fixed index. LOCATE it by walking
// pairs: for i:=0; i+1<len(top.Content); i+=2 { if top.Content[i].Value=="customCommands" { seq:=top.Content[i+1] } }.
// A throwaway test that hardcoded root.Content[0].Content[1] mutated the WRONG node (gui's value) and left the
// marker untouched (research/yaml-node-api.md §2 — reproduced + fixed empirically).

// CRITICAL (the marker is on the VALUE scalar, matched by substring): stagecoach's entry is identified by a
// LineComment "# stagecoach-integration" on item.Content[1] (the `key` VALUE scalar "<c-a>", NOT the key NAME).
// yaml.v3 KEEPS the "# " prefix in the stored LineComment, so detect via strings.Contains(lc, "stagecoach-integration")
// (substring — robust to the prefix). The marker — NOT the binding — is the idempotency identity (FR-I3b).

// CRITICAL (lazygit's identity is the marker; --key is the binding): contrast git-alias, whose identity IS
// the alias name (alias.<name>). For lazygit, Remove deletes the MARKER entry regardless of --key (you might
// install with --key '<c-b>' and remove with the default). Register --key on install AND remove for UI
// symmetry + resetIntegrateFlags parity (mirrors --alias-name), but document that remove targets the marker.

// CRITICAL (Parse MUST refuse on any error): yaml.Unmarshal returns non-nil for corrupt YAML (a plain error
// OR *yaml.TypeError which has a PARTIAL node but is still non-nil). Any non-nil err ⇒ return it ⇒ Apply
// HARD-REFUSES ("refused to write <path>: parse error"). Branch on err!=nil, never on a nil-node assumption.

// CRITICAL (Validate is a clean local probe): Target.Validate must NOT rely on prior Parse state (the Target
// contract). Implement as `var n yaml.Node; return yaml.Unmarshal(data, &n)` on a throwaway — Apply calls it
// on freshly-written bytes; a non-nil err ⇒ Apply restores the backup (FR-I3e).

// CRITICAL (enc.Close() is REQUIRED): yaml.NewEncoder buffers; you MUST call enc.Close() and check its error
// to flush the final bytes. Forgetting it truncates the output silently (research/yaml-node-api.md §7).

// CRITICAL (test isolation — never the real config): lazygit's real config path is ~/.config/lazygit/config.yml
// (or --print-config-dir). Tests MUST construct lazygitEntry with an EXPLICIT tmp configPath (bypassing the
// resolver) and write fixtures into t.TempDir(). newLazygitEntry() (the prod factory) resolves the path; tests
// build &lazygitEntry{configPath: tmpfile, key: "<c-a>"} directly. NEVER let a test resolve+write the real path.

// GOTCHA (defaultEntries + resetIntegrateFlags are SHARED with git-alias): after T2.S1, defaultEntries returns
// []Entry{ newGitAliasEntry() } and resetIntegrateFlags resets --alias-name. T2.S2 APPENDS lazygit to BOTH —
// additive, each owning its own flag/entry. Do NOT modify git-alias's lines. defaultEntries is called
// per-command after flag parse, so flagLazygitKey is populated when newLazygitEntry() runs.

// GOTCHA (config-path resolution is best-effort + subprocess): resolveLazygitConfigPath() runs
// `lazygit --print-config-dir` (short -cd; NO --config-dir), trims stdout → <dir>/config.yml. On ANY error
// (lazygit absent, non-zero exit, empty) fall back to os.UserConfigDir() + "lazygit/config.yml" (cross-platform:
// ~/.config on Linux, ~/Library/Application Support on macOS, %AppData% on Windows). Never fatal — ConfigPath
// may return the fallback with DETECTED ✗ in `list`. NOTE: os.UserConfigDir() returns %AppData% (Roaming) on
// Windows, not %LOCALAPPDATA% — a minor known limitation; --print-config-dir covers the installed case.

// GOTCHA (lazygitEntry.Install does NOT hand-roll preview/confirm/backup): it delegates ALL of FR-I3 to
// integrate.Apply (unlike git-alias, which bypasses Apply and hand-rolls its own preview because git owns the
// file). lazygit's Install builds ApplyOptions{Target: &lazygitTarget{key:...}, Action: ActionUpsert, Yes, Out,
// Confirm} and maps ApplyResult→InstallResult. Do NOT write a backup/atomic/diff helper — Apply has them.

// GOTCHA (remove leaves customCommands: [] if it was the only entry): deleting the marked item and re-encoding
// an empty sequence yields `customCommands: []`, which lazygit tolerates and round-trips (verified). Prefer
// leaving the empty seq (minimal edit, HasEntry correctly false) over deleting the customCommands key (larger
// diff). FR-I6 "restore entry-absence" = HasEntry()==false, satisfied either way.

// GOTCHA (no transitive-confusion): go.sum already carries `go.yaml.in/yaml/v3 v3.0.4` (a cobra/pflag pull).
// That is a DIFFERENT module path. The DIRECT dependency to add is `gopkg.in/yaml.v3` — import EXACTLY
// "gopkg.in/yaml.v3" (never "go.yaml.in/yaml/v3").
```

## Implementation Blueprint

### Data models and structure

```go
// === internal/cmd/integrate_lazygit.go ===
package cmd // import "github.com/dustin/stagecoach/internal/cmd"

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dustin/stagecoach/internal/integrate"
	"gopkg.in/yaml.v3" // v3.0.1 — archived upstream (2025); pinned for the Node API (HeadComment/LineComment/
	                  // FootComment) used for comment-preserving customCommands upsert. Verified 2026-07-02.
)

const (
	lazygitTargetName = "lazygit"
	defaultLazygitKey = "<c-a>"          // FR-I5: the binding the originating commit-pi workflow used (PRD §2.1)
	lazygitMarker     = "stagecoach-integration" // the LineComment substring that identifies stagecoach's entry

	// lazygitSchemaVerified = "2026-07-02 against lazygit v0.62.2" // record in a comment (FR-D5)
)

// entryTpl is the ONE stagecoach customCommands entry, as a one-item YAML sequence document. The %s is the key
// binding. The `# stagecoach-integration` marker rides on the `key` VALUE scalar (LineComment) — stagecoach's
// idempotency identity (FR-I3b), independent of the binding. Field names verified against lazygit v0.62.2
// (external_deps.md §1): `output` (not the older subprocess/showOutput), `loadingText` valid, context `files`.
var entryTpl = `- key: '%s' # stagecoach-integration
  context: 'files'
  command: 'stagecoach'
  loadingText: 'Generating commit message…'
  output: 'none'
  description: 'stagecoach: AI commit'
`

var flagLazygitKey string // --key (local on integrateInstallCmd AND integrateRemoveCmd; mirrors --alias-name)

func init() {
	// Register --key on BOTH leaves for UI symmetry + resetIntegrateFlags parity with --alias-name.
	// (Remove targets the MARKER entry regardless of --key — the marker is stagecoach's identity; --key is the
	// binding stagecoach writes. Documented in docs/cli.md.) Shared var; default "" → resolved to "<c-a>".
	integrateInstallCmd.Flags().StringVar(&flagLazygitKey, "key", "",
		"lazygit key binding to install (default: <c-a>)")
	integrateRemoveCmd.Flags().StringVar(&flagLazygitKey, "key", "",
		"lazygit key binding (default: <c-a>; remove targets the marked stagecoach entry)")
}

// ---------------------------------------------------------------------------
// lazygitTarget — S1's integrate.Target adapter over yaml.v3's Node API.
// Owns ONLY the surgical node edit; integrate.Apply owns the no-mangle envelope
// (parse-first, preview-diff, confirm, backup, atomic write, validate+restore).
// ---------------------------------------------------------------------------

// lazygitTarget implements integrate.Target for lazygit's config.yml (PRD §9.21 FR-I5). key is the binding
// stagecoach writes; root is the parsed document (populated by Parse). Stateful: Parse populates root;
// HasEntry/Upsert/Remove read/mutate it; Validate is a clean local probe (no Parse reliance).
type lazygitTarget struct {
	key  string      // the binding ("<c-a>"); never "" (factory resolves)
	root *yaml.Node  // parsed DocumentNode (nil before Parse; the create-path may build it fresh in Upsert)
}

// Marker returns the idempotency-identity substring (FR-I3b).
func (t *lazygitTarget) Marker() string { return lazygitMarker }

// Parse loads config.yml bytes into the node tree. Any error (incl. *yaml.TypeError) ⇒ refuse-to-write.
func (t *lazygitTarget) Parse(data []byte) error {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return err // Apply HARD-REFUSES: "refused to write <path>: parse error: <err>"
	}
	t.root = &root
	return nil
}

// HasEntry reports whether the marker-identified entry is present in the parsed tree.
func (t *lazygitTarget) HasEntry() bool { return t.findMarkedItem() != nil }

// Upsert returns new bytes with the stagecoach entry inserted (absent) or replaced (present). Surgical:
// only the marker entry changes semantically (incidental whole-doc normalization is possible — architecture
// §2 — and surfaced by Apply's preview-diff).
func (t *lazygitTarget) Upsert() ([]byte, error) {
	if t.root == nil { // empty/missing file path — build a fresh document holding just the entry
		t.root = &yaml.Node{Kind: yaml.DocumentNode,
			Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	}
	top := t.topMap()
	entry, err := t.newEntryNode()
	if err != nil {
		return nil, err
	}
	seq := t.locateSeq(top)
	if seq == nil { // customCommands key absent on an existing top map — append key + a new seq (preserves other keys)
		seq = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		top.Content = append(top.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "customCommands"}, seq)
	}
	// REPLACE the marked item if present (idempotent — never duplicate), else APPEND.
	replaced := false
	for i, it := range seq.Content {
		if isStagecoachItem(it) {
			seq.Content[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		seq.Content = append(seq.Content, entry)
	}
	return t.encode(t.root)
}

// Remove returns new bytes with the marker entry deleted. If customCommands becomes empty, it is left as
// `customCommands: []` (lazygit tolerates it; minimal edit; HasEntry==false). No marked entry ⇒ bytes unchanged.
func (t *lazygitTarget) Remove() ([]byte, error) {
	if t.root == nil {
		return nil, nil // nothing parsed (shouldn't happen — Apply only calls Remove when HasEntry)
	}
	top := t.topMap()
	seq := t.locateSeq(top)
	if seq == nil {
		return t.encode(t.root) // no customCommands at all — unchanged
	}
	for i, it := range seq.Content {
		if isStagecoachItem(it) {
			seq.Content = append(seq.Content[:i], seq.Content[i+1:]...)
			break // at most one marked entry (Upsert guarantees it)
		}
	}
	return t.encode(t.root)
}

// Validate re-parses on a throwaway (clean, side-effect-free; no Parse reliance). Apply calls it on written bytes.
func (t *lazygitTarget) Validate(data []byte) error {
	var n yaml.Node
	return yaml.Unmarshal(data, &n)
}

// --- lazygitTarget helpers ---

// topMap returns the top-level mapping (root.Content[0] for a DocumentNode), or a fresh map for empty input.
func (t *lazygitTarget) topMap() *yaml.Node {
	if t.root != nil && t.root.Kind == yaml.DocumentNode && len(t.root.Content) > 0 {
		return t.root.Content[0]
	}
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	if t.root == nil {
		t.root = &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{m}}
	} else {
		t.root.Content = []*yaml.Node{m}
	}
	return m
}

// locateSeq walks the top mapping's Content IN PAIRS to find the customCommands sequence (NEVER a hardcoded idx).
func (t *lazygitTarget) locateSeq(top *yaml.Node) *yaml.Node {
	for i := 0; i+1 < len(top.Content); i += 2 {
		if top.Content[i].Kind == yaml.ScalarNode && top.Content[i].Value == "customCommands" &&
			top.Content[i+1].Kind == yaml.SequenceNode {
			return top.Content[i+1]
		}
	}
	return nil
}

// findMarkedItem returns the marked list item (or nil). The marker is on item.Content[1] (the `key` value).
func (t *lazygitTarget) findMarkedItem() *yaml.Node {
	if t.root == nil {
		return nil
	}
	top := t.topMap()
	seq := t.locateSeq(top)
	if seq == nil {
		return nil
	}
	for _, it := range seq.Content {
		if isStagecoachItem(it) {
			return it
		}
	}
	return nil
}

// findKeyItem returns an UNMARKED item whose key value == key (for Foreign status detection), or nil.
func (t *lazygitTarget) findKeyItem(key string) *yaml.Node {
	if t.root == nil {
		return nil
	}
	seq := t.locateSeq(t.topMap())
	if seq == nil {
		return nil
	}
	for _, it := range seq.Content {
		if it.Kind != yaml.MappingNode || len(it.Content) < 2 {
			continue
		}
		// Content[0]="key" name, Content[1]=value; only the first field is the binding
		if it.Content[0].Value == "key" && it.Content[1].Value == key && !isStagecoachItem(it) {
			return it
		}
	}
	return nil
}

// newEntryNode builds the stagecoach entry from entryTpl and returns its single MappingNode.
func (t *lazygitTarget) newEntryNode() (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(fmt.Sprintf(entryTpl, t.key)), &doc); err != nil {
		return nil, fmt.Errorf("build stagecoach entry: %w", err)
	}
	return doc.Content[0].Content[0], nil // DocumentNode → SequenceNode → the one MappingNode item
}

// encode re-encodes the document with SetIndent(2) (lazygit convention; default is 4). enc.Close() is REQUIRED.
func (t *lazygitTarget) encode(root *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// isStagecoachItem reports whether a sequence item is stagecoach's (marker on the `key` value scalar).
func isStagecoachItem(item *yaml.Node) bool {
	return item.Kind == yaml.MappingNode && len(item.Content) >= 2 &&
		strings.Contains(item.Content[1].LineComment, lazygitMarker)
}

// ---------------------------------------------------------------------------
// lazygitEntry — S2's integrate.Entry. Wires lazygitTarget through integrate.Apply.
// ---------------------------------------------------------------------------

// lazygitEntry implements integrate.Entry for the lazygit target (PRD §9.21 FR-I5/I6). configPath is the
// resolved config.yml (prod: via resolveLazygitConfigPath; tests: an explicit tmp path). key is the binding.
type lazygitEntry struct {
	configPath string // resolved; tests set this directly to avoid touching the real config
	key        string // resolved (never "" — newLazygitEntry resolves "" → "<c-a>")
}

// newLazygitEntry builds the entry for the current invocation (reads the resolved --key + config path).
func newLazygitEntry() *lazygitEntry {
	key := flagLazygitKey
	if key == "" {
		key = defaultLazygitKey
	}
	return &lazygitEntry{configPath: resolveLazygitConfigPath(), key: key}
}

func (e *lazygitEntry) Name() string { return lazygitTargetName }

// Detect — FR-I2: lazygit requires lazygit on $PATH.
func (e *lazygitEntry) Detect(context.Context) error {
	if _, err := exec.LookPath("lazygit"); err != nil {
		return fmt.Errorf("lazygit not found on PATH: %w", err)
	}
	return nil
}

// ConfigPath — the resolved config.yml (list CONFIG column; display-only, best-effort).
func (e *lazygitEntry) ConfigPath(context.Context) (string, error) {
	if e.configPath == "" {
		e.configPath = resolveLazygitConfigPath()
	}
	return e.configPath, nil
}

// Status — FR-I1: marker present → Installed; unmarked item with our key → Foreign; else NotInstalled.
func (e *lazygitEntry) Status(ctx context.Context) (integrate.Status, error) {
	data, err := os.ReadFile(e.resolvedPath())
	if err != nil {
		return integrate.StatusNotInstalled, nil // missing file ⇒ not installed (not an error)
	}
	tgt := &lazygitTarget{key: e.key}
	if perr := tgt.Parse(data); perr != nil {
		return integrate.StatusNotInstalled, nil // unparseable ⇒ not installed (install will refuse)
	}
	if tgt.HasEntry() {
		return integrate.StatusInstalled, nil
	}
	if tgt.findKeyItem(e.key) != nil {
		return integrate.StatusForeign, nil // a conflicting (unmarked) entry binds our key
	}
	return integrate.StatusNotInstalled, nil
}

// Install — FR-I5: drive the no-mangle protocol (integrate.Apply) to upsert the marker entry.
func (e *lazygitEntry) Install(ctx context.Context, opts integrate.InstallOptions) (integrate.InstallResult, error) {
	tgt := &lazygitTarget{key: e.key}
	res, err := integrate.Apply(ctx, integrate.ApplyOptions{
		Path:    e.resolvedPath(),
		Target:  tgt,
		Action:  integrate.ActionUpsert,
		Yes:     opts.Yes,
		Out:     opts.Out,
		Confirm: opts.Confirm,
	})
	if err != nil {
		return integrate.InstallResult{}, err // dispatch wraps via exitcode.New; parse-refusal surfaces here
	}
	return integrate.InstallResult{Outcome: res.Outcome, Target: e.Name(), Path: res.Path, Backup: res.Backup}, nil
}

// Remove — FR-I6: drive Apply with ActionRemove (deletes only the marker entry; restores entry-absence).
func (e *lazygitEntry) Remove(ctx context.Context, opts integrate.RemoveOptions) (integrate.RemoveResult, error) {
	tgt := &lazygitTarget{key: e.key}
	res, err := integrate.Apply(ctx, integrate.ApplyOptions{
		Path:    e.resolvedPath(),
		Target:  tgt,
		Action:  integrate.ActionRemove,
		Yes:     opts.Yes,
		Out:     opts.Out,
		Confirm: opts.Confirm,
	})
	if err != nil {
		return integrate.RemoveResult{}, err
	}
	return integrate.RemoveResult{Outcome: res.Outcome, Target: e.Name(), Path: res.Path, Backup: res.Backup}, nil
}

// resolvedPath returns the config path (e.configPath, resolving once if empty).
func (e *lazygitEntry) resolvedPath() string {
	if e.configPath == "" {
		e.configPath = resolveLazygitConfigPath()
	}
	return e.configPath
}

// resolveLazygitConfigPath discovers lazygit's config dir via `lazygit --print-config-dir` (short -cd; NO
// --config-dir), else falls back to the platform default (<userConfigDir>/lazygit/config.yml). Best-effort;
// never fatal (returns the fallback even on error). external_deps.md §1 (VERIFIED 2026-07-02 v0.62.2).
func resolveLazygitConfigPath() string {
	if out, err := exec.Command("lazygit", "--print-config-dir").Output(); err == nil {
		if dir := strings.TrimSpace(string(out)); dir != "" {
			return filepath.Join(dir, "config.yml")
		}
	}
	if ucd, err := os.UserConfigDir(); err == nil { // ~/.config (Linux), ~/Library/Application Support (macOS), %AppData% (Windows)
		return filepath.Join(ucd, "lazygit", "config.yml")
	}
	// last resort: HOME/.config/lazygit/config.yml
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "lazygit", "config.yml")
	}
	return "config.yml" // unreachable in practice; Apply would then fail with a clear path error
}
```

```go
// === internal/cmd/integrate.go — S2's file, the ONE edit (defaultEntries) ===
// BEFORE (after T2.S1):
//   var defaultEntries = func() []integrate.Entry {
//       return []integrate.Entry{ newGitAliasEntry() }
//   }
// AFTER (T2.S2):
var defaultEntries = func() []integrate.Entry {
	return []integrate.Entry{ newGitAliasEntry(), newLazygitEntry() } // git-alias (T2.S1), lazygit (T2.S2)
}

// === internal/cmd/integrate_test.go — S2's file, the ONE edit (resetIntegrateFlags) ===
// T2.S2 APPENDS inside the existing resetIntegrateFlags (after T2.S1's --alias-name block):
//   // T2.S2: reset --key flag on install and remove
//   flagLazygitKey = ""
//   for _, c := range []*cobra.Command{integrateInstallCmd, integrateRemoveCmd} {
//       if f := c.Flags().Lookup("key"); f != nil && f.Changed { f.Changed = false }
//   }
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD gopkg.in/yaml.v3 v3.0.1 to go.mod
  - RUN: `go get gopkg.in/yaml.v3@v3.0.1 && go mod tidy`. Confirm go.mod gains `require gopkg.in/yaml.v3 v3.0.1`
    and go.sum gains its hash. NOTE: a TRANSITIVE `go.yaml.in/yaml/v3 v3.0.4` may already be in go.sum (cobra
    pull) — that is a DIFFERENT module; leave it. The DIRECT import is "gopkg.in/yaml.v3".
  - ADD a source comment at the import (in integrate_lazygit.go) noting archived-upstream + the verification
    date (FR-D5): "// gopkg.in/yaml.v3 v3.0.1 — archived upstream (2025); pinned for the Node API. Verified 2026-07-02."

Task 2: CREATE internal/cmd/testdata/lazygit/{golden_input.yml, golden_corrupt.yml}
  - golden_input.yml: a HAND-MAINTAINED, COMMENTED lazygit config — a `gui:` block (with a user comment), a
    `customCommands:` sequence with ONE pre-existing entry (e.g. `key: 'b', command: "echo existing",
    context: 'files'`), a `git:` block (with nested keys), and a blank line between sections (to surface the
    whole-doc-normalization behavior). NO stagecoach entry (the install fixture). Keep it realistic + commented.
  - golden_corrupt.yml: malformed YAML (e.g. an unclosed flow `[unclosed` or bad indent) for the parse-refusal test.
  - PLACEMENT: internal/cmd/testdata/lazygit/ (Go ignores testdata/ in builds; tests read via
    os.ReadFile("testdata/lazygit/golden_input.yml") — cwd is the package dir during `go test`).

Task 3: CREATE internal/cmd/integrate_lazygit.go — lazygitTarget + lazygitEntry + --key + init()
  - IMPLEMENT lazygitTarget (all 6 integrate.Target methods) per the Blueprint's data-models block:
    Marker/Parse/HasEntry/Upsert/Remove/Validate + helpers (topMap/locateSeq/findMarkedItem/findKeyItem/
    newEntryNode/encode) + isStagecoachItem. USE the pair-walking locateSeq; the substring marker match;
    enc.SetIndent(2)+enc.Close(); the empty-seq-on-remove behavior; the create-if-missing (empty root +
    absent-key) paths.
  - IMPLEMENT lazygitEntry (all 6 integrate.Entry methods): Name/Detect/ConfigPath/Status/Install/Remove +
    newLazygitEntry factory + resolveLazygitConfigPath + resolvedPath. Install/Remove build a lazygitTarget
    and call integrate.Apply (ActionUpsert/ActionRemove), mapping ApplyResult→InstallResult/RemoveResult.
  - IMPLEMENT `var flagLazygitKey string` + init() registering --key on integrateInstallCmd AND
    integrateRemoveCmd (StringVar, default "").
  - GOTCHA: lazygitEntry does NOT hand-roll preview/confirm/backup — Apply owns them. Status is best-effort
    (missing/unparseable file ⇒ NotInstalled, nil err). resolvedPath caches configPath. Tests construct the
    entry with an explicit configPath (bypassing the resolver) — so resolvedPath must use e.configPath as-is.
  - DEPENDENCIES: internal/integrate (Target/Apply/Outcome/Action/ApplyOptions/Entry/Status/InstallOptions/
    RemoveOptions/InstallResult/RemoveResult/ConfirmFunc) + gopkg.in/yaml.v3. Imports: stdlib (bytes/context/
    fmt/os/os/exec/path/filepath/strings) + those two.
  - PLACEMENT: internal/cmd/integrate_lazygit.go (isolated from S2's integrate.go; mirrors git-alias's single file).

Task 4: EDIT internal/cmd/integrate.go — defaultEntries seam (S2's + T2.S1's file)
  - CHANGE defaultEntries to return []integrate.Entry{ newGitAliasEntry(), newLazygitEntry() } (append
    newLazygitEntry() after git-alias's entry — T2.S1 lands git-alias first). Do NOT touch anything else.
  - GOTCHA: defaultEntries is called per-command after flag parse (runIntegrateList/Install/Remove), so
    flagLazygitKey is populated when newLazygitEntry() runs. newLazygitEntry resolves the config path
    best-effort (lazygit --print-config-dir → fallback); never fatal.

Task 5: EDIT internal/cmd/integrate_test.go — resetIntegrateFlags (S2's + T2.S1's file)
  - APPEND to the existing resetIntegrateFlags (after T2.S1's --alias-name block): `flagLazygitKey = ""` +
    reset the Changed bit on BOTH integrateInstallCmd and integrateRemoveCmd's "key" flag (see Blueprint).
  - GOTCHA: resetIntegrateFlags is shared with git-alias — append ONLY the --key block; do NOT modify the
    --yes or --alias-name lines.

Task 6: CREATE internal/cmd/integrate_lazygit_test.go — golden-file + matrix + wiring
  - HELPER: newIsolatedLazygitEntry(t, key) builds &lazygitEntry{configPath: tmpfile, key: resolvedKey}
    where tmpfile = filepath.Join(t.TempDir(), "config.yml") + a fixed-bool Confirm (for confirm-flow tests).
    NEVER resolves the real path. Plus readFixture(t, name) → os.ReadFile("testdata/lazygit/"+name).
  - lazygitTarget TESTS (call methods directly; no Apply, no cobra):
    * TestLazygitTarget_ParseGolden: Parse(golden_input) → nil err; HasEntry()==false; locateSeq finds the
      pre-existing 'b' entry.
    * TestLazygitTarget_Upsert_AddsEntrySemantically: Parse(golden) → Upsert → re-parse the output → assert
      (a) marked entry present with key/context/command/output/loadingText/description == defaults;
      (b) the pre-existing 'b' entry is PRESERVED (its key+command+context unchanged); (c) other top-level
      blocks (gui/git) present with their comments; (d) the marker substring present. (Assert SEMANTIC content
      via re-parse, NOT byte-equality of unrelated nodes — whole-doc normalization is allowed.)
    * TestLazygitTarget_Upsert_IdempotentStableRoundTrip: take Upsert's output, Parse it, Upsert AGAIN →
      bytes.Equal(firstOutput, secondOutput)==true (stagecoach's own writes round-trip stably ⇒ Apply NoChange).
    * TestLazygitTarget_Upsert_ReplaceNotDuplicate: Parse an input that ALREADY has the marker entry → Upsert
      → re-parse → exactly ONE marked item (the customCommands sequence length unchanged; the entry replaced).
    * TestLazygitTarget_Remove_DeletesOnlyMarked: Parse an input with the marker entry + a sibling 'b' entry →
      Remove → re-parse → marker GONE, the 'b' entry PRESERVED (byte content), customCommands still present.
    * TestLazygitTarget_Remove_EmptySeq: Parse an input whose ONLY entry is the marked one → Remove → re-parse
      OK; customCommands present as empty (`[]` or absent) — assert HasEntry()==false on a fresh Parse.
    * TestLazygitTarget_Remove_NoMarkerUnchanged: Parse golden (no marker) → Remove → output re-parses; no
      stagecoach entry present (remove is a no-op on a clean file).
    * TestLazygitTarget_ParseCorruptRefuses: Parse(golden_corrupt) → non-nil error (Apply will refuse).
    * TestLazygitTarget_Validate: Validate(wellFormedBytes)==nil; Validate(corruptBytes)!=nil (clean probe).
    * TestLazygitTarget_CreateIfMissing_EmptyFile: Parse([]byte("")) then Upsert → output is a valid config
      with exactly the stagecoach entry (re-parse; HasEntry true; one customCommands item).
    * TestLazygitTarget_CreateIfMissing_AbsentKey: Parse a config with NO customCommands key (but other top
      keys) → Upsert → re-parse → customCommands now present with the entry AND all original top-level keys
      preserved (the absent-key append must not rebuild the whole mapping).
    * TestLazygitTarget_CustomKey: Upsert with key "<c-s>" → the marked entry's key value == "<c-s>".
  - Apply/Entry TESTS (real Apply over a tmp file; YES to skip confirm where noted):
    * TestLazygitEntry_Install_Creates: missing tmpfile → Install(Yes) → OutcomeCreated; file exists; re-read
      → marked entry present; Backup=="" (created, not modified).
    * TestLazygitEntry_Install_Updated: pre-write golden to tmpfile → Install(Yes) → OutcomeUpdated; Backup
      starts with ".stagecoach-backup." (a backup was written for the existing-file modification); entry present.
    * TestLazygitEntry_Install_IdempotentNoChange: Install(Yes) then Install(Yes) AGAIN → second is
      OutcomeNoChange (no second write; backup unchanged).
    * TestLazygitEntry_Install_DeclineWritesNothing: Confirm stub returns false → OutcomeDeclined; tmpfile
      UNCHANGED (or absent); no backup.
    * TestLazygitEntry_Install_ConfirmReceivesDiff: capture the `diff` arg the Confirm stub receives → contains
      "+  - key: '<c-a>'" / "stagecoach-integration" / "customCommands" (a real unified diff from git diff --no-index).
    * TestLazygitEntry_Install_CorruptRefuses: write golden_corrupt to tmpfile → Install(Yes) → non-nil error
      containing "parse error" / "refused to write"; tmpfile UNCHANGED (byte-identical to the corrupt input).
    * TestLazygitEntry_Remove_Removed: install then Remove(Yes) → OutcomeRemoved; re-read → no marked entry;
      sibling entries (if any) intact; Backup written (existing-file modification).
    * TestLazygitEntry_Remove_NoChange: remove on a file with no marker → OutcomeNoChange; nothing written.
    * TestLazygitEntry_Status_States: missing file → NotInstalled; install → Installed; hand-write an unmarked
      item with key "<c-a>" → Foreign; clean golden → NotInstalled.
    * TestLazygitEntry_Detect: present → nil; t.Setenv("PATH","") → non-nil error containing "lazygit not found".
    * TestLazygitEntry_ConfigPath: with a tmp configPath set → returns it verbatim (no subprocess).
  - WIRING TESTS (saveRootState/restoreRootState + resetIntegrateFlags + swap defaultEntries + SetArgs + Execute):
    * TestIntegrateInstall_Lazygit_Execute: swap defaultEntries to []Entry{ newLazygitEntry() } BUT override the
      config path — since newLazygitEntry resolves the real path, instead build the registry manually with an
      isolated entry, OR set flagLazygitKey and rely on a tmp config via a test-only seam. SIMPLEST: test the
      Execute wiring by asserting `integrate install lazygit --yes` routes to the lazygit entry (Detect gate +
      dispatch) using an isolated GIT/PATH + a factory that points at a tmp path. If a config-path override
      seam is needed, add a test-only `defaultEntriesWith(path)` helper in the _test.go (package cmd can access
      unexported fields). Assert exit 0 + a stdout status line ("Installed lazygit" / "No changes").
    * TestIntegrateLazygitKeyFlag: `integrate install lazygit --yes --key '<c-s>'` → the written entry's key
      value == "<c-s>" (read the tmp config + assert).
    * TestIntegrateRemove_Lazygit_Execute: install then `integrate remove lazygit --yes` → exit 0; entry gone.
    - NOTE: because newLazygitEntry resolves the REAL path via lazygit --print-config-dir, Execute-level tests
      that actually WRITE must redirect the path. Preferred approach: in the _test.go, construct the registry
      directly (`integrate.NewRegistry([]integrate.Entry{ &lazygitEntry{configPath: tmp, key:"<c-a>"} })`) and
      call dispatchInstall/dispatchRemove (the PURE functions) rather than Execute — this avoids touching the
      real config AND tests the real dispatch path. Reserve Execute-only tests for --key flag PARSING (assert
      flagLazygitKey is read) without writing. (Mirrors git-alias's "call methods directly; Execute for flag wiring".)
  - FOLLOW pattern: internal/integrate/protocol_test.go (blockTarget fake + Apply matrix — the test style for
    Target/Apply); integrate_test.go (saveRootState/resetIntegrateFlags/swap defaultEntries); config_test.go
    (rootCmd.SetArgs + Execute + substring asserts).
  - GOTCHA: EVERY test that writes must use a tmpfile (t.TempDir()) — the real ~/.config/lazygit/config.yml
    is NEVER touched. Reset flagLazygitKey via resetIntegrateFlags between Execute tests.

Task 7: EDIT docs/cli.md — lazygit target section (Mode A, EXTENDS S2's integrate group section)
  - ADD a `### \`lazygit\` target` subsection within/after S2's `integrate` group section (and after T2.S1's
    git-alias subsection if present): what it does (adds ONE `customCommands` entry to lazygit's config.yml
    via a comment-preserving YAML round-trip; the entry runs `stagecoach` on `<c-a>` in the files panel with
    `output: 'none'` so you stay in the UI — US8); the entry defaults table; the `--key <k>` override; config
    discovery order (`lazygit --print-config-dir` → platform default); the no-mangle protocol (parse-first,
    preview, backup, validate+restore) + that a corrupt config is refused; the marker (`# stagecoach-integration`)
    identity + idempotency (re-install = no-op; replace-not-duplicate); uninstall symmetry (remove deletes
    only the stagecoach entry, leaving the rest); what `list` shows (DETECTED ✓ when on $PATH; STATUS; CONFIG).
    One example each for install/remove + `--key`.
  - GOTCHA: S2 wrote the `integrate list/install/remove` GROUP subsections; T2.S1 adds git-alias; T2.S2 ADDS the
    lazygit TARGET detail + the --key flag + the customCommands entry shape (PRD §15.5). Do NOT rewrite S2's
    group section — extend it. Match the existing per-target heading + example-block + flag-table style.
  - INCLUDE (per PRD §15.5) the documented entry:
    ```yaml
    customCommands:
      - key: '<c-a>'                       # stagecoach-integration
        context: 'files'
        command: 'stagecoach'
        loadingText: 'Generating commit message…'
        output: 'none'
        description: 'stagecoach: AI commit'
    ```

Task 8: VERIFY build/test/lint (go.mod gains exactly yaml.v3)
  - go build ./... ; go test ./internal/cmd/... -run 'Lazygit' -v ; go test ./internal/integrate/... -v ;
    go test ./... ; go vet ./... ; golangci-lint run ; gofmt -l internal/cmd.
  - Confirm `git diff go.mod` shows exactly the one `gopkg.in/yaml.v3 v3.0.1` require line added (no other
    dep churn). Confirm NO test touched ~/.config/lazygit/config.yml (isolation audit — see Level 4).
```

### Implementation Patterns & Key Details

```go
// locateSeq — THE pair-walking locate (never hardcode the index; research/yaml-node-api.md §2).
func (t *lazygitTarget) locateSeq(top *yaml.Node) *yaml.Node {
	for i := 0; i+1 < len(top.Content); i += 2 { // PAIRS: Content[i]=key-scalar, Content[i+1]=value-node
		if top.Content[i].Kind == yaml.ScalarNode && top.Content[i].Value == "customCommands" &&
			top.Content[i+1].Kind == yaml.SequenceNode {
			return top.Content[i+1]
		}
	}
	return nil
}

// isStagecoachItem — the marker is on item.Content[1] (the `key` VALUE scalar), matched by substring
// (yaml keeps the "# " prefix in LineComment). The marker — not the binding — is the identity.
func isStagecoachItem(item *yaml.Node) bool {
	return item.Kind == yaml.MappingNode && len(item.Content) >= 2 &&
		strings.Contains(item.Content[1].LineComment, lazygitMarker)
}

// newEntryNode — build from the YAML template (simplest; exact desired serialization), take the single item.
func (t *lazygitTarget) newEntryNode() (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(fmt.Sprintf(entryTpl, t.key)), &doc); err != nil {
		return nil, fmt.Errorf("build stagecoach entry: %w", err)
	}
	return doc.Content[0].Content[0], nil // DocumentNode → SequenceNode → the one MappingNode
}

// encode — SetIndent(2) (lazygit convention; default 4); enc.Close() is REQUIRED (buffers).
func (t *lazygitTarget) encode(root *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil { return nil, err }
	if err := enc.Close(); err != nil { return nil, err }
	return buf.Bytes(), nil
}

// lazygitEntry.Install — delegate ALL of FR-I3 to integrate.Apply; map the result.
func (e *lazygitEntry) Install(ctx context.Context, opts integrate.InstallOptions) (integrate.InstallResult, error) {
	tgt := &lazygitTarget{key: e.key}
	res, err := integrate.Apply(ctx, integrate.ApplyOptions{
		Path: e.resolvedPath(), Target: tgt, Action: integrate.ActionUpsert,
		Yes: opts.Yes, Out: opts.Out, Confirm: opts.Confirm,
	})
	if err != nil { return integrate.InstallResult{}, err }
	return integrate.InstallResult{Outcome: res.Outcome, Target: e.Name(), Path: res.Path, Backup: res.Backup}, nil
}

// resetIntegrateFlags extension (appended to S2's helper, after T2.S1's --alias-name block):
//   flagLazygitKey = ""
//   for _, c := range []*cobra.Command{integrateInstallCmd, integrateRemoveCmd} {
//       if f := c.Flags().Lookup("key"); f != nil && f.Changed { f.Changed = false }
//   }
```

### Integration Points

```yaml
PROVIDES (to the integrate command surface — already wired by S2):
  - lazygitEntry                        # the second concrete integrate.Entry (Name="lazygit")
  - lazygitTarget                       # an integrate.Target (yaml.v3 adapter) — driven by protocol.Apply
  - defaultEntries now [git-alias, lazygit]   # `integrate list` shows both rows; install/remove drive both
  - --key flag                          # local on integrate install + remove
  - gopkg.in/yaml.v3 v3.0.1             # the new direct dependency (pinned, archived-upstream-noted)

CONSUMES (do NOT re-implement):
  - integrate.Target / Apply / ApplyOptions / ApplyResult / Outcome / Action (S1) — lazygit is the FILE-EDITING
    target that exercises the FULL protocol (unlike git-alias which bypasses Apply).
  - integrate.Entry / Status / InstallOptions / RemoveOptions / InstallResult / RemoveResult / Registry (S2).
  - integrate.ConfirmFunc / DefaultConfirm / BackupPath (S1).
  - S2's defaultEntries seam + dispatchInstall/dispatchRemove/printIntegrateList/formatInstallResult/
    formatRemoveResult + resetIntegrateFlags + the cobra integrate group.
  - T2.S1's defaultEntries append (git-alias lands first; lazygit appends second).

OUT OF SCOPE (owned by sibling subtasks — do NOT implement):
  - S1 (done): the no-mangle protocol engine — lazygit CONSUMES Apply; it does NOT re-implement parse/backup/
    atomic/validate/diff (Apply has them all).
  - S2 (done): the command surface, registry, dispatch, the integrate group.
  - T2.S1 (parallel): git-alias — co-resident; both append to defaultEntries/resetIntegrateFlags additively.
  - P1.M7: README coherence (this subtask edits ONLY docs/cli.md; the README quick-start snippet swap rides
    with P1.M7.T1.S1 per the work-item description).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
gofmt -w internal/cmd/integrate_lazygit.go internal/cmd/integrate_lazygit_test.go
go build ./...            # lazygitTarget + lazygitEntry compile against internal/integrate + gopkg.in/yaml.v3
go vet ./...
golangci-lint run
# go.mod MUST show exactly the one new require:
git diff go.mod | grep -E '^\+.*gopkg.in/yaml.v3' && echo "yaml.v3 added OK"
git diff go.mod | grep -E '^\+' | grep -v 'gopkg.in/yaml.v3' && echo "WARN: unexpected go.mod churn" || echo "go.mod churn clean OK"
# Expected: zero errors; go.mod gains exactly gopkg.in/yaml.v3 v3.0.1.
```

### Level 2: Unit Tests (Component Validation)

```bash
go test ./internal/cmd/... -run LazygitTarget -v
# Expected: ParseGolden, Upsert_AddsEntrySemantically, Upsert_IdempotentStableRoundTrip, Upsert_ReplaceNotDuplicate,
#   Remove_DeletesOnlyMarked, Remove_EmptySeq, Remove_NoMarkerUnchanged, ParseCorruptRefuses, Validate,
#   CreateIfMissing_EmptyFile, CreateIfMissing_AbsentKey, CustomKey — all pass.

go test ./internal/cmd/... -run LazygitEntry -v
# Expected: Install_Creates, Install_Updated, Install_IdempotentNoChange, Install_DeclineWritesNothing,
#   Install_ConfirmReceivesDiff, Install_CorruptRefuses, Remove_Removed, Remove_NoChange, Status_States,
#   Detect, ConfigPath — all pass; no test touches ~/.config/lazygit/config.yml.

go test ./internal/cmd/... -run 'Integrate.*Lazygit' -v   # dispatch + --key wiring
# Expected: dispatchInstall/dispatchRemove drive the lazygit entry; --key parsed + applied.

go test ./internal/integrate/... -v    # S1/S2 regression — Apply + Registry unchanged
go test ./...                          # full suite — no regression (config/providers/hook/git/git-alias)
# Expected: all pass; the REAL lazygit config is never touched (isolation via tmp configPath).
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach

# list now shows BOTH targets (git-alias + lazygit):
/tmp/stagecoach integrate list
# Expected: a lazygit row — DETECTED ✓ (if on $PATH), STATUS not installed, CONFIG = the resolved config.yml.

# Real install against a THROWAWAY config (NEVER your real ~/.config/lazygit/config.yml in this manual check).
# Override the path by running in an env where `lazygit --print-config-dir` points at a tmp dir, OR by using
# the test harness. Simplest manual check: temporarily point LAZYGIT at a tmp config via a HOME override:
export HOME=/tmp/sh-home; mkdir -p "$HOME/.config/lazygit"
cat > "$HOME/.config/lazygit/config.yml" <<'YML'
gui:
  showRandomTip: false
customCommands:
  - key: 'b'
    command: "echo existing"
    context: 'files'
YML
# (lazygit --print-config-dir may still resolve to the real XDG path; if so, copy the fixture there instead.)
/tmp/stagecoach integrate install lazygit --yes
# Expected: OutcomeCreated/Updated; re-read the config → the stagecoach entry present; the 'b' entry PRESERVED;
# comments/formatting otherwise intact.
/tmp/stagecoach integrate install lazygit --yes    # again → "No changes for lazygit (already installed)."
/tmp/stagecoach integrate remove lazygit --yes     # → "Removed lazygit integration"; 'b' entry still there.

# corrupt config → refuse
echo 'customCommands: [unclosed' > "$HOME/.config/lazygit/config.yml"
/tmp/stagecoach integrate install lazygit --yes; echo "exit=$?"
# Expected: exit 1, stderr "refused to write ...: parse error"; config UNCHANGED on disk.

# --key override
rm -f "$HOME/.config/lazygit/config.yml"
/tmp/stagecoach integrate install lazygit --yes --key '<c-s>'
grep -q "key: '<c-s>'" "$HOME/.config/lazygit/config.yml" && echo "OK: custom key written"
unset HOME
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Comment-preservation audit — install into the golden fixture, then assert EVERY non-stagecoach node survived:
export HOME=/tmp/sh-audit; mkdir -p "$HOME/.config/lazygit"
cp internal/cmd/testdata/lazygit/golden_input.yml "$HOME/.config/lazygit/config.yml"
/tmp/stagecoach integrate install lazygit --yes
python3 - <<'PY'   # (or eyeball it) — assert the pre-existing entry + gui/git blocks are intact post-install
import yaml, sys
d = yaml.safe_load(open("/tmp/sh-audit/.config/lazygit/config.yml"))
assert d["customCommands"][0]["key"] == "b", "pre-existing entry lost!"
assert d["gui"]["showRandomTip"] is False, "gui block mangled!"
print("OK: non-stagecoach content preserved")
PY
unset HOME

# Idempotency stress — install 10×; the config stabilizes after the first (NoChange, no drift, one entry):
export HOME=/tmp/sh-idem; mkdir -p "$HOME/.config/lazygit"; rm -f "$HOME/.config/lazygit/config.yml"
for i in $(seq 1 10); do /tmp/stagecoach integrate install lazygit --yes >/dev/null; done
N=$(grep -c "stagecoach-integration" "$HOME/.config/lazygit/config.yml")
[ "$N" = "1" ] && echo "OK: exactly one stagecoach entry after 10 installs" || echo "FAIL: $N entries (drift!)"
unset HOME

# Isolation audit — confirm NO test wrote the real lazygit config:
REAL="$HOME/.config/lazygit/config.yml"
BEFORE="$(cat "$REAL" 2>/dev/null | sha256sum)"
go test ./internal/cmd/... -run Lazygit -count=1
AFTER="$(cat "$REAL" 2>/dev/null | sha256sum)"
[ "$BEFORE" = "$AFTER" ] && echo "OK: real lazygit config untouched by tests" || echo "FAIL: real config modified!"

# Remove restores entry-absence + sibling survival (FR-I6):
go test ./internal/cmd/... -run 'LazygitTarget_Remove_DeletesOnlyMarked|LazygitEntry_Remove_Removed' -v -count=1
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...`; `go test ./...`; `go vet ./...`; `golangci-lint run`; `gofmt -l` clean.
- [ ] `go.mod` gains EXACTLY `gopkg.in/yaml.v3 v3.0.1` (no other churn); source comment notes archived + date.
- [ ] `lazygitTarget` implements all six `integrate.Target` methods via the yaml.v3 Node API; `lazygitEntry`
      implements all six `integrate.Entry` methods and drives `integrate.Apply` (does NOT hand-roll FR-I3).
- [ ] Tests isolate via explicit tmp `configPath` (t.TempDir()) — the real `~/.config/lazygit/config.yml` is
      NEVER touched (isolation audit passes).

### Feature Validation
- [ ] `integrate list` shows `lazygit` (DETECTED ✓ when on $PATH; STATUS not-installed/installed/foreign;
      CONFIG = resolved config.yml).
- [ ] `integrate install lazygit` writes a comment-preserving entry (re-parse: marker present, defaults
      correct, ALL other entries/blocks/comments preserved); re-run is NoChange (idempotent).
- [ ] A corrupt `config.yml` → install REFUSES with a parse-error message and writes NOTHING (exit 1).
- [ ] `integrate remove lazygit` deletes ONLY the marker entry (sibling entries intact; empty seq left if it
      was the only entry); re-run is NoChange (FR-I6 uninstall symmetry).
- [ ] `--key '<c-s>'` writes that binding; the marker (`# stagecoach-integration`) is the idempotency identity.
- [ ] Decline (user N / non-TTY no --yes) writes nothing; Outcome=Declined.
- [ ] A backup (`<file>.stagecoach-backup.<ts>`) is written for existing-file modifications (not for creates).

### Code Quality Validation
- [ ] `lazygitTarget` locates `customCommands` by PAIR-WALKING (never a hardcoded index); the marker is on the
      value scalar, matched by substring; `enc.SetIndent(2)` + `enc.Close()` used.
- [ ] `lazygitEntry.Install`/`Remove` delegate to `integrate.Apply` and map `ApplyResult`→`Install`/`RemoveResult`
      (reuses Outcome/Backup/Path); does NOT re-implement parse/backup/atomic/validate/diff.
- [ ] The only edits to S2/T2.S1's files are `defaultEntries` (append newLazygitEntry) + `resetIntegrateFlags`
      (append the --key block) — minimal, additive, non-conflicting with git-alias.
- [ ] lazygit logic is isolated in `integrate_lazygit.go`/`_test.go` + `testdata/lazygit/` (low merge-friction).
- [ ] Follows existing conventions: single-file concrete Entry (git-alias), local-flag-on-install+remove
      (--alias-name), exitcode routing (providers.go), t.TempDir + assert tests (protocol_test.go/hookspath).

### Documentation & Deployment
- [ ] docs/cli.md has the `lazygit` target subsection (Mode A) extending S2's integrate group section: entry
      defaults table, `--key`, config discovery order, no-mangle+backup behavior, marker+idempotency, uninstall
      symmetry, what `list` shows, one install/remove example, the §15.5 entry shape.
- [ ] No README.md edit (P1.M7 owns the coherence sweep; the quick-start snippet swap rides with P1.M7.T1.S1).

---

## Anti-Patterns to Avoid

- ❌ Don't hardcode the `customCommands` index — walk `topMap.Content` IN PAIRS (`Content[i]`=key,
  `Content[i+1]`=value). The throwaway test that indexed `Content[1]` mutated gui's value, not customCommands,
  and left the marker untouched (research/yaml-node-api.md §2 — reproduced empirically).
- ❌ Don't match the marker on the `key` NAME scalar — it's on the `key` VALUE scalar (`item.Content[1]`), via
  `strings.Contains(LineComment, "stagecoach-integration")` (yaml keeps the "# " prefix). The marker — not the
  binding — is the identity.
- ❌ Don't hand-roll preview/confirm/backup/atomic-write/validate — `integrate.Apply` owns ALL of FR-I3.
  `lazygitEntry.Install` builds `ApplyOptions{Target: &lazygitTarget{…}, Action: ActionUpsert, …}` and maps the
  result. (Contrast git-alias, which bypasses Apply and hand-rolls its preview because `git config` owns the
  file — lazygit is the FILE-EDITING target that uses the FULL machinery.)
- ❌ Don't forget `enc.Close()` — `yaml.NewEncoder` buffers; skipping `Close()` truncates the output silently.
- ❌ Don't forget `enc.SetIndent(2)` — the default encoder indent is 4; lazygit configs use 2.
- ❌ Don't treat `*yaml.TypeError` as success — it's a non-nil error with a PARTIAL node. Any `Unmarshal` error
  ⇒ `Parse` returns it ⇒ Apply HARD-REFUSES ("refused to write …: parse error"). Branch on `err != nil`.
- ❌ Don't make `Validate` depend on prior `Parse` state — it's a clean local probe (`var n yaml.Node; return
  yaml.Unmarshal(data, &n)`) on a throwaway (the Target contract; Apply calls it on written bytes).
- ❌ Don't delete the whole `customCommands` key on remove — leave `customCommands: []` (verified: lazygit
  tolerates an empty list; minimal edit; `HasEntry()` correctly false). Deleting the key is a larger diff.
- ❌ Don't let `Status` error on a missing/unparseable file — return `NotInstalled, nil` (best-effort; install
  will refuse with the real error). `Status` is independent of `Detect`.
- ❌ Don't let any test write the real `~/.config/lazygit/config.yml` — construct `lazygitEntry` with an
  explicit tmp `configPath` (bypassing the resolver). Execute-level WRITE tests should call the pure
  `dispatchInstall`/`dispatchRemove` with a manually-built isolated registry, not `Execute` (which uses the
  real-path-resolving factory). Reserve `Execute` for `--key` flag PARSING assertions.
- ❌ Don't import `go.yaml.in/yaml/v3` — that's the cobra transitive (a different module). The DIRECT dep is
  `gopkg.in/yaml.v3`. Import EXACTLY `"gopkg.in/yaml.v3"`.
- ❌ Don't add a second new dependency — only `gopkg.in/yaml.v3`. If you reach for go-toml/spf13/etc., STOP.
- ❌ Don't rewrite S2's/T2.S1's integrate.go/integrate_test.go — the only edits are `defaultEntries` (append
  `newLazygitEntry()`) + `resetIntegrateFlags` (append the `--key` block). Everything lazygit owns lives in its
  own `integrate_lazygit.go`/`_test.go` + `testdata/lazygit/`.
- ❌ Don't confuse `--key` (the binding stagecoach WRITES) with the entry identity (the MARKER). Remove targets
  the marker regardless of `--key`. Register `--key` on install AND remove for UI symmetry + resetIntegrateFlags
  parity, but document remove-by-marker.
- ❌ Don't assert byte-equality of UNRELATED nodes in the golden round-trip test — yaml.v3 re-encodes the whole
  doc (may normalize blank lines / inline-comment spacing). Assert SEMANTIC content via re-parse (the marker
  entry's fields, sibling entries preserved, other blocks present + their comments). Idempotency (NoChange) is
  asserted on stagecoach's OWN writes round-tripping stably, which they do.

---

## Confidence Score

**9/10** — one-pass success likelihood is high. The contract is precisely specified and the riskiest part
(the yaml.v3 Node surgery) is VERIFIED EMPIRICALLY in research/yaml-node-api.md (locate-by-pair-walk with the
hardcoded-index trap reproduced+avoided; marker-on-value-scalar substring match; insert/replace/remove;
SetIndent(2)+Close; create-if-missing empty+absent-key; corrupt-refuse; empty-seq-on-remove; whole-doc
normalization + idempotency-via-stable-round-trip). The consumed contracts are read from the COMPLETED S1/S2
SOURCE (Target/Apply/Outcome/Action/Entry/Status/Options/Result/Registry + the defaultEntries/
resetIntegrateFlags seams + dispatch + formatters), and the co-resident sibling (T2.S1 git-alias) is named as
the exact skeleton to mirror (`--key`↔`--alias-name`, `newLazygitEntry`↔`newGitAliasEntry`, append to both
shared seams). lazygit is the FILE-EDITING target that exercises Apply's FULL machinery — so lazygit itself
implements almost no I/O/safety logic (Apply owns parse/backup/atomic/validate/diff), keeping its surface to
the YAML node edit + the Entry wiring. Residual uncertainty: (a) the Execute-level write test needs a
config-path override seam (prescribed: call dispatchInstall/dispatchRemove with a manually-built isolated
registry rather than Execute); (b) `os.UserConfigDir()` returns `%AppData%` (not `%LOCALAPPDATA%`) on Windows
— a minor best-effort fallback limitation that `--print-config-dir` covers for the installed case. Both are
documented and bounded; neither blocks one-pass success.
