# P1.M4.T1.S2 — Codebase & Pattern Research

Research date: 2026-07-02. Scope: the `integrate list|install|remove` **command surface + target
registry + detection gating** (FR-I1/I2). This subtask CONSUMES S1's protocol engine (`protocol.Apply` +
`Target` + `Outcome`) and PROVIDES the dispatch surface T2.S1 (git-alias) / T2.S2 (lazygit) plug into.

---

## 1. What S1 (the contract) provides — `internal/integrate/protocol.go`

S1's PRP is the authoritative contract. The symbols my command surface consumes:

- `integrate.Target` interface: `Marker()/Parse(data)/HasEntry()/Upsert()/Remove()/Validate(data)`.
  → Implemented by **lazygit** (T2.S2); NOT by git-alias (delegates to `git config`, FR-I4).
- `integrate.Apply(ctx, ApplyOptions) (ApplyResult, error)` — the FR-I3 (a)–(g) no-mangle engine.
- `integrate.ApplyOptions{Path, Target, Action, Yes, Out, Confirm}` — `Confirm` is `ConfirmFunc`;
  `nil` ⇒ `DefaultConfirm` (TTY-gated y/N; non-TTY auto-decline). `Yes` skips the prompt.
- `integrate.ApplyResult{Outcome, Path, Backup}` — `Outcome` ∈ {Created,Updated,Removed,Declined,NoChange}.
- `integrate.Outcome` (+ `String()`) — **the shared reporting vocabulary**. My `InstallResult.Outcome`
  reuses this exact type so lazygit's Apply maps 1:1 and git-alias maps its own result into the same set.
- `integrate.ConfirmFunc` = `func(out io.Writer, path, diff string) bool`.
- `integrate.BackupPath(path, unixTs)` → `<file>.stagecoach-backup.<ts>`.

**Key takeaway:** S1's `Outcome` enum is the unified status both targets report. My registry/types must NOT
re-invent an outcome enum — import `integrate.Outcome`.

---

## 2. The registry precedent — `internal/provider/registry.go` (the shape to mirror)

`provider.Registry` is the closest structural analog: a compiled-in, name-keyed, sorted list.

- `NewRegistry(userOverrides map[string]Manifest)` — **constructs fresh** (no global mutable state).
- `Get(name) (Manifest, bool)` — existence probe for unknown-target refusal.
- `List() []Manifest` — **sorted ascending by Name** (deterministic for `list`). Returns a fresh slice.
- `IsInstalled(m) bool` — probes `m.DetectCommand()` via `exec.LookPath`. → My `Entry.Detect(ctx)` is the
  generalization (git-alias: git-on-path; lazygit: lazygit-on-path). `exec.LookPath` is the probe.
- `preferredBuiltins` slice + `DefaultProvider(installed)` — the preference-order pattern.

**Pattern to extract:** Registry = `{manifests map[string]Entry}`; `NewRegistry([]Entry)`; `Get`/`List`
(sorted). **Take the target list as a constructor arg** (pure, testable) — the cmd layer supplies the slice.

---

## 3. The cmd-layer precedent — `internal/cmd/hook.go` + `internal/cmd/providers.go` (cobra wiring)

These two are the templates. Confirmed conventions:

- **cobra commands live in `internal/cmd/`, registered via `init()` on `rootCmd`** (ZERO edits to root.go).
  `rootCmd.AddCommand(<groupCmd>)` in the file's `init()`. providers.go L~140, hook.go `init()`.
- **Group command**: `Use:`, `Short:`, `Long:`, `SilenceErrors:true`, `SilenceUsage:true`, NO `RunE` (bare
  `stagecoach <group>` prints help). Leaf commands have `RunE`.
- **PersistentPreRunE no-op to SKIP config.Load** (hook.go): `PersistentPreRunE: func(*cobra.Command,
  []string) error { return nil }`. cobra runs only the NEAREST PreRunE, so this OVERRIDES root's
  config.Load. **integrate needs this** — it edits user dotfiles (gitconfig, lazygit config), works
  OUTSIDE a git repo, and must NOT trigger config.Load's first-run bootstrap write (FR-B3). Same rationale
  as hook. providers.go does NOT define its own (it NEEDS cfg.Providers); integrate does NOT need cfg.
- **exitcode routing**: `return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", err))`. Never
  `os.Exit`. `cmd.OutOrStdout()` / `cmd.ErrOrStderr()` for writers. `cmd.Context()` for ctx.
- **list table** (providers.go `printProvidersList`): `text/tabwriter.NewWriter(w, 0,0,2,' ',0)`, header
  row, one row per entry, ✓/✗ for detected, `(default)` marker column. Takes `io.Writer` (testable).
  **`printIntegrateList` mirrors this exactly.**
- **Args validators**: `cobra.NoArgs` (list/status), `cobra.ExactArgs(1)` (providers show). For
  `install <target>…`/`remove <target>…` (multiple, ≥1): `cobra.MinimumNArgs(1)`.

**Pure-dispatch seam (testability):** providers.go splits `runProvidersList` (cobra RunE: builds registry,
calls print) from `printProvidersList(w, reg, default)` (pure, takes writer+registry). I mirror this:
`runIntegrateList` (RunE) → `printIntegrateList(w, ctx, reg)` (pure); `runIntegrateInstall` (RunE) →
`dispatchInstall(ctx, reg, targets, opts, out)` (pure). **Tests call the pure funcs with a fake-seeded
Registry — no cobra needed for the core logic.**

---

## 4. The registration seam — how T2 plugs in (NO edit to a shared library file by T2's targets)

The work item's OUTPUT contract: "command surface dispatching to targets registered by P1.M4.T2.S1/S2."
The cleanest seam that keeps `internal/integrate` a pure library AND avoids global mutable state:

- **`internal/cmd/integrate.go` owns `defaultEntries()`** — a package-level `var defaultEntries = func()
  []integrate.Entry { return nil }` (function var so it constructs fresh each call, mirroring
  provider.NewRegistry). S2 ships it returning **nil** (empty registry — `list` shows a header-only table,
  `install <x>` → unknown-target). **T2.S1/T2.S2 append their `Entry` impls here** (one line each) — this is
  the documented, single registration point. T2 owns its Entry impl in its own file; the ONLY cross-file
  touch is appending to `defaultEntries`.
- **Why not `init()`-based `integrate.Register()`?** That needs blank imports in `internal/cmd/integrate.go`
  (which MY file would own) + global mutable state in `internal/integrate`. The provider precedent avoids
  globals; `defaultEntries()` matches it and is trivially testable (tests swap the var for fakes).
- **Tests inject fakes** by setting `defaultEntries = func() []integrate.Entry { return []Entry{...fakes...} }`
  (restored in defer), OR by calling the pure `dispatch*`/`printIntegrateList` funcs with a hand-built
  `integrate.NewRegistry(fakes)`. The latter is the PRIMARY test surface (no Execute needed).

---

## 5. The Entry interface — abstracting file-edit (lazygit) vs command-delegate (git-alias)

Critical asymmetry (FR-I4 vs FR-I5): **lazygit installs via `protocol.Apply`** (file edit); **git-alias
installs via `git config --global alias.<name> '!stagecoach'`** (git does the edit; FR-I3 machinery
"unnecessary"). BUT both still **preview + confirm** (FR-I3c applies to git-alias too: "the command and
resulting alias are still shown and confirmed"). So the Entry must own its OWN install/remove mechanics
while sharing the **confirm controls** (`Yes`, `Out`, `Confirm`).

Resolved design — `Entry` carries the 4 registry-facing fields the work item names PLUS install/remove:

```go
type Entry interface {
    Name() string
    Detect(ctx context.Context) error          // nil=tool present; non-nil=absent (FR-I2 gate)
    ConfigPath(ctx context.Context) (string, error) // FR-I1 list column + error context
    Status(ctx context.Context) (Status, error)     // FR-I1: NotInstalled|Installed|Foreign
    Install(ctx context.Context, opts InstallOptions) (InstallResult, error)
    Remove(ctx context.Context, opts RemoveOptions) (RemoveResult, error)
}
```

- `InstallOptions{Yes bool; Out io.Writer; Confirm integrate.ConfirmFunc}` — shared controls.
  lazygit: passes straight to `ApplyOptions{Yes, Out, Confirm}`. git-alias: honors `Yes`/`Confirm` for its
  OWN confirm (the alias preview). Target-specific flags (--alias-name, --key) are read by T2's
  `defaultEntries()` entry-construction (the cmd layer builds the Entry with resolved flag values), so the
  Entry interface stays narrow. **`Confirm==nil` ⇒ `integrate.DefaultConfirm`** (the shared y/N).
- `InstallResult{Outcome integrate.Outcome; Target, Path, Backup string}` — Outcome is S1's enum (unified).
  lazygit: `Outcome: res.Outcome, Path: res.Path, Backup: res.Backup`. git-alias: maps git-config result.
- `Status` enum: `StatusNotInstalled`/`StatusInstalled`/`StatusForeign` (+ `String()` → "not installed"/
  "installed"/"foreign" exactly per FR-I1). **Distinct from `hook.Status`** (None/Stagecoach/Foreign) —
  integrate has its own; do NOT reuse hook's.

Sentinels: `ErrUnknownTarget` (Get miss), `ErrToolNotDetected` (Detect fail). `errors.Is`-chainable.

---

## 6. Detection gating + batch dispatch semantics (FR-I2)

Work item: "a target whose tool is absent is LISTED but install exits 1 with a note."
- **list**: shows ALL targets (detected or not) — DETECTED=✗ is informational, not a refusal.
- **install/remove**: for each named target, **gate on Detect BEFORE Install/Remove**. Absent ⇒ print a
  note to stderr ("target X requires <tool>, which is not on $PATH") + mark failed. **Continue with the
  remaining targets** (best-effort: `install git-alias lazygit` still installs git-alias if lazygit is
  absent). **Exit 1 if ANY target failed** (detection-gate OR install error). Decline/NoChange are NOT
  errors (exit 0). This mirrors batch-tool conventions and the work item's per-target "note".
- git-alias's Detect = "git on $PATH" (always true for stagecoach; FR-I2 "requires only git itself").

---

## 7. exitcode + ui dependencies (confirmed)

- `exitcode.New(exitcode.Error, err)` → exit 1. `exitcode.For(err)` in main. Detection-gate / unknown-target
  / install-error all map to `exitcode.Error` (1). Decline/NoChange → nil (exit 0).
- `integrate.DefaultConfirm` (S1) already imports `internal/ui.IsTerminal` for the non-TTY auto-decline.
  My cmd layer passes `opts.Confirm = nil` (⇒ DefaultConfirm) in prod; tests inject a fixed-bool Confirm.
- **integrate does NOT import `internal/config`** (skips config.Load). No Config() use. Mirrors hook.

---

## 8. Test patterns (confirmed from hook_test.go / providers_test.go / root_test.go)

- Shared helpers in `internal/cmd/root_test.go`: `loadEnvSetup(t)` (HOME/XDG/repo isolation),
  `chdir(t, dir)`, `saveRootState(t)`/`restoreRootState(t, ...)` (rootCmd out/err/RunE), `writeConfigFile`,
  `initRepo(t, dir)`. `resetHookFlags(t)` (hook_test.go) is the per-command-flag-reset template → I add
  `resetIntegrateFlags(t)` for `flagIntegrateYes`.
- **Primary tests = pure dispatch**: build `integrate.NewRegistry([]Entry{fakes...})`, call
  `dispatchInstall(ctx, reg, []string{"..."}, opts, &out, &errBuf)`, assert exit-1-on-detection-gate,
  status substrings, batch continue, decline/no-change exit 0. NO Execute/cobra needed.
- **Fake Entry** (in-package test type in `integrate_test.go`): configurable `detectErr`, `status`,
  `installResult`, `installErr`, recorded call log. Proves gating + reporting without real targets.
- **Execute-level wiring tests**: `integrate list` (header-only empty table), `integrate install bogus` →
  exit 1 + "unknown target", `--yes` flag parse. Use saveRootState/restoreRootState + SetArgs pattern.

---

## 9. Docs (Mode A) — docs/cli.md

Structure confirmed: `## Subcommands` section with per-command `### ` headings (hook install/uninstall/
status/exec at L62–132; providers list/show at L133–156). The integrate group adds `### `integrate list``
/ `### `integrate install <target>…`` / `### `integrate remove <target>…`` under Subcommands, BEFORE the
`## Exit codes` section (L203). Content per the work item: targets table (git-alias, lazygit; gitui blocked
→ FUTURE_SPEC.md), detection gating (absent tool = listed but install exits 1), and a one-paragraph
no-mangle protocol summary (preview+confirm, backup, auto-restore) cross-referencing FR-I3.

---

## 10. Scope fences (do NOT implement — owned by siblings)

- **S1 (contract, parallel):** `Target`, `Apply`, `Outcome`, `ConfirmFunc`, `DefaultConfirm`, `BackupPath`.
- **T2.S1 (git-alias):** the `gitAliasEntry` Entry impl + `--alias-name` flag + its line in `defaultEntries`.
  Delegates edit to `git config` (FR-I4; external_deps.md §7: `--get` read-back, strip leading `!`).
- **T2.S2 (lazygit):** the `lazygitEntry` Entry impl (yaml.v3 Node API) + `--key` flag + its line in
  `defaultEntries` + adds `gopkg.in/yaml.v3` to go.mod. (external_deps.md §1/§2: `output` field,
  `lazygit --print-config-dir`, SetIndent(2), protocol-not-library is the no-mangle guarantee.)
- **NOT this subtask:** any concrete Entry, yaml.v3, the actual git-config/lazygit-config editing.
  S2 ships the surface + registry + fakes only; `defaultEntries()` returns nil.
