name: "P1.M4.T1.S2 — `integrate list|install|remove` command surface with detection gating"
description: |
  Implement the COBRA COMMAND SURFACE + TARGET REGISTRY + DETECTION GATING for `stagecoach integrate`
  (PRD §9.21 FR-I1/I2, §15.3). Three leaf commands on an `integrate` group: `list` (TARGET/DETECTED/
  STATUS/CONFIG table), `install <target>…` and `remove <target>…` (explicit targets, ≥1, multiple
  allowed). Consumes P1.M4.T1.S1's protocol engine (`protocol.Apply` + `Target` + `Outcome`) and
  PROVIDES the dispatch surface that P1.M4.T2.S1 (git-alias) and P1.M4.T2.S2 (lazygit) register their
  concrete targets into. Ships NO concrete target — the registry is seeded EMPTY in this subtask
  (`defaultEntries()` returns nil); T2 appends its `Entry` impls as the single registration seam.
  Detection gating (FR-I2): a target whose tool is absent is LISTED (DETECTED=✗) but `install`/`remove`
  for it print a note and exit 1, while remaining named targets are still attempted (best-effort batch).
  `--yes` (persistent on the `integrate` parent) flows into `InstallOptions.Yes`/`RemoveOptions.Yes`,
  which each target honors (lazygit → `ApplyOptions.Yes`; git-alias → skips its own confirm). Unknown
  target → exit 1 with "see `integrate list`". Tests use FAKE `Entry` impls (no real git-alias/lazygit)
  exercising detection gating, status reporting (NotInstalled/Installed/Foreign), unknown-target refusal,
  batch continue-on-failure, decline/no-change exit 0, and Execute-level cobra wiring. **Consumer of
  S1; provider for T2.S1/S2.** Adds the [Mode A] docs/cli.md `integrate` group section. NO new third-party
  dependency (yaml.v3 is T2.S2's; git-alias delegates to `git config`).

---

## Goal

**Feature Goal**: Ship the user-facing `stagecoach integrate list|install|remove` command surface and the
pluggable `Entry`/`Registry` abstraction that dispatches to it, with detection gating (FR-I2) and status
reporting (FR-I1) fully working — so that when T2.S1 (git-alias) and T2.S2 (lazygit) append their `Entry`
implementations to the single `defaultEntries()` seam, the commands light up with zero further cmd-layer
work. The surface must be provably correct BEFORE any concrete target exists: detection gating refuses to
install a target whose tool is absent (exit 1 + note), `list` reports detected/config-path/status for every
registered target, unknown targets are rejected with a discoverable error, and `--yes` flows through to
the (future) target install/remove confirming machinery.

**Deliverable**:
1. `internal/integrate/registry.go` — `Status` enum (NotInstalled/Installed/Foreign + `String()`); the
   `Entry` interface (Name/Detect/ConfigPath/Status/Install/Remove); `InstallOptions`/`RemoveOptions`
   (shared `Yes`/`Out`/`Confirm` controls); `InstallResult`/`RemoveResult` (carrying S1's
   `integrate.Outcome`); `Registry` (`NewRegistry([]Entry)`, `Get(name) (Entry,bool)`, `List() []Entry`
   sorted by Name); sentinel errors `ErrUnknownTarget`, `ErrToolNotDetected`.
2. `internal/cmd/integrate.go` — the cobra `integrate` command group (parent: no-op `PersistentPreRunE`
   to skip config.Load, persistent `--yes` flag) + `list`/`install`/`remove` leaves; the `defaultEntries`
   seam (returns nil in S2); pure `printIntegrateList`/`dispatchInstall`/`dispatchRemove` functions.
3. `internal/cmd/integrate_test.go` — an in-package fake `Entry` + the dispatch matrix (detection gating,
   status reporting, unknown-target, batch continue, decline/no-change, `--yes`) + Execute-level wiring.
4. `docs/cli.md` — [Mode A] `integrate` group section under `## Subcommands` (targets table, detection
   gating, one-paragraph no-mangle protocol summary).

**Success Definition**: `integrate list` prints a deterministic TARGET/DETECTED/STATUS/CONFIG table over
the registry (empty ⇒ header only); `install <target>…`/`remove <target>…` dispatch per-target with
detection gating (absent tool ⇒ note + exit 1, remaining targets still attempted, exit 1 iff any failed);
unknown target ⇒ exit 1 + "see `integrate list`"; `--yes` populates `InstallOptions.Yes`; the `Entry`
interface + `Registry` are the exact contract T2.S1/S2 implement. `go build ./...`, `go test ./...`,
`go vet ./...`, `golangci-lint run`, `gofmt -l` all green; `go.mod` gains NO new `require`.

## User Persona

**Target User**: the "plan-holder" (PRD §7.1) who lives in lazygit and types `git <thing>` all day — they
want `git stagecoach` and a `<c-a>` keybind wired in one command, but their overriding fear is "will this
trash my hand-tuned dotfiles?" (PRD §9.21 lead-in). This subtask delivers the *front door* — `integrate
list` to see what's available + detected, `install <target>` to opt into exactly the targets they name
(no "install everything" surprise), and `remove` to undo — while the no-mangle guarantee itself is S1's
engine that the surface drives.

**Use Case**: `stagecoach integrate list` → see lazygit detected, git-alias always-available. `stagecoach
integrate install git-alias lazygit` → each is previewed/confirmed/backed-up (via S1's protocol, once T2
plugs in). If lazygit isn't installed, it's still listed but install prints a note and exits 1.

**User Journey**: `list` (survey) → `install <target>…` (diff → y/N → backup → done) → later `remove
<target>…` (restore to pre-stagecoach). `--yes` for scripts.

**Pain Points Addressed**: no magic "install all" (explicit targets only — delta_prd.md R4); a missing
tool is reported, not silently skipped or crashing; the surface is uniform across targets so adding a
third target (future) is one Entry + one registration line.

## Why

- **PRD §9.21 FR-I1** (the surface: `list` table + `install`/`remove` explicit targets) and **FR-I2**
  (detection gating: absent tool listed but install exits 1 with a note).
- **delta_prd.md R4**: "explicit targets only (no install-everything default); detection-gated: a target
  whose tool is absent is LISTED but install exits 1 with a note (FR-I2; git-alias requires only git).
  v2.1 targets are exactly git-alias and lazygit (gitui is blocked upstream — FUTURE_SPEC.md)."
- **PRD §15.3**: `stagecoach integrate list|install <target>…|remove <target>…` subcommand reference.
- **Scope fences**: CONSUMES S1's protocol engine + `Outcome`/`ConfirmFunc`/`DefaultConfirm`; PROVIDES the
  dispatch surface + the `Entry`/`Registry` contract consumed by T2.S1 (git-alias) and T2.S2 (lazygit).
  Does NOT implement any concrete target, does NOT add yaml.v3, does NOT edit gitconfig/lazygit-config.

## What

A cobra command group (`integrate`) with three leaves over a pluggable target registry. The registry holds
`Entry` records (the abstraction T2 implements); the commands build the registry from a `defaultEntries()`
seam, then `list` renders a table and `install`/`remove` dispatch per-target with detection gating. The
whole surface is testable with fake `Entry` impls before any real target exists.

### Success Criteria

- [ ] `internal/integrate/registry.go`: `Status` (NotInstalled/Installed/Foreign + exact `String()`);
      `Entry` interface (Name/Detect/ConfigPath/Status/Install/Remove); `InstallOptions`/`RemoveOptions`
      (`Yes bool; Out io.Writer; Confirm integrate.ConfirmFunc`); `InstallResult`/`RemoveResult`
      (`Outcome integrate.Outcome; Target, Path, Backup string`); `Registry` (`NewRegistry([]Entry)`,
      `Get(name)(Entry,bool)`, `List() []Entry` sorted ascending by Name); `ErrUnknownTarget`,
      `ErrToolNotDetected` sentinels.
- [ ] `internal/cmd/integrate.go`: `integrateCmd` parent (no-op `PersistentPreRunE` to skip config.Load —
      works outside a repo, no FR-B3 bootstrap; persistent `--yes` flag); `integrateListCmd`
      (`cobra.NoArgs`), `integrateInstallCmd`/`integrateRemoveCmd` (`cobra.MinimumNArgs(1)`); registered
      via `init()` on `rootCmd` (ZERO edits to root.go — providers.go/hook.go pattern).
- [ ] `var defaultEntries = func() []integrate.Entry { return nil }` — the single registration seam
      (documented; T2.S1/S2 append). `runIntegrateList/Install/Remove` build `integrate.NewRegistry
      (defaultEntries())` and delegate to pure `printIntegrateList`/`dispatchInstall`/`dispatchRemove`.
- [ ] Detection gating (FR-I2): `dispatchInstall`/`dispatchRemove` call `entry.Detect(ctx)` BEFORE
      Install/Remove; a non-nil Detect ⇒ note to stderr naming the target/tool + mark failed; remaining
      named targets are still attempted; exit 1 (`exitcode.Error`) iff any target failed or was unknown.
      Decline/NoChange outcomes are NOT errors (exit 0).
- [ ] `printIntegrateList(w, ctx, reg)`: tabwriter table `TARGET\tDETECTED\tSTATUS\tCONFIG` — one row per
      `reg.List()` (sorted), DETECTED ✓/✗ via Detect, STATUS via Status().String(), CONFIG via
      ConfigPath (or "—" on error/empty). Deterministic; takes `io.Writer`.
- [ ] Unknown target: `Get` miss ⇒ stderr "unknown target <x>; see `stagecoach integrate list`" + exit 1.
- [ ] `internal/cmd/integrate_test.go`: fake `Entry` (configurable detect/status/install outcome/err +
      call log) + dispatch matrix (detection-gate-exit-1-with-note, status-reporting per state,
      unknown-target, batch-continue-on-failure, decline+no-change-exit-0, `--yes` flows to opts) +
      Execute-level wiring (`list` empty header, `install bogus` exit 1, `--yes` parses).
- [ ] `docs/cli.md`: [Mode A] `integrate list`/`install`/`remove` subsections under `## Subcommands`
      (targets table: git-alias, lazygit; gitui blocked → FUTURE_SPEC.md; detection gating; no-mangle
      protocol summary: preview+confirm, backup, auto-restore).
- [ ] `go build ./...` + `go test ./...` + `go vet ./...` + `golangci-lint run` + `gofmt -l` clean;
      `go.mod` UNCHANGED (no new `require`).

## All Needed Context

### Context Completeness Check

_This PRP names the exact `Entry` method set + the exact `InstallOptions`/`Result` shapes (reusing S1's
`Outcome`/`ConfirmFunc`/`DefaultConfirm`), the exact `Registry` API (mirroring `provider.Registry`), the
exact cobra wiring (parent no-op `PersistentPreRunE` to skip config.Load — the hook.go precedent; `init()`
registration; `cobra.MinimumNArgs(1)`), the exact detection-gating + batch-dispatch semantics (per-target
Detect gate, continue-on-failure, exit 1 iff any failed, Decline/NoChange exit 0), the `defaultEntries()`
registration seam for T2, the tabwriter table layout (the providers.go `printProvidersList` precedent),
the fake-Entry test vehicle + matrix, and the docs/cli.md placement + content. An implementer with no
prior codebase knowledge can build it from this document + codebase access._

### Documentation & References

```yaml
- docfile: plan/005_c38aa48290f0/P1M4T1S2/research/codebase-patterns.md
  why: Condensed research — S1's consumed contract (Target/Apply/Outcome/ConfirmFunc/DefaultConfirm), the
       provider.Registry shape to mirror, the hook.go/providers.go cmd-layer conventions (no-op
       PersistentPreRunE to skip config.Load; init() registration; tabwriter list; pure-dispatch seam), the
       Entry interface design resolving the file-edit(lazygit) vs command-delegate(git-alias) asymmetry, the
       defaultEntries() registration seam, detection-gating + batch semantics, test patterns, and scope fences.
  section: all
  critical: |
    integrate SKIPS config.Load (no-op PersistentPreRunE, like hook) — it edits user dotfiles, works outside
    a repo, and must NOT trigger FR-B3's first-run bootstrap. Reuse S1's integrate.Outcome as the unified
    result vocabulary (do NOT re-invent an outcome enum). The Entry's Confirm==nil ⇒ integrate.DefaultConfirm
    (TTY-gated). integrate.Status is DISTINCT from hook.Status (own enum, own strings).

- docfile: plan/005_c38aa48290f0/P1M4T1S1/PRP.md
  why: THE CONTRACT this subtask consumes. §"Data models" (Target, Apply, ApplyOptions, ApplyResult, Outcome,
       ConfirmFunc, DefaultConfirm, BackupPath), §"Integration Points / PROVIDES". Treat as authoritative.
  section: "Data models and structure; Integration Points (PROVIDES)"
  critical: |
    Outcome = {Created,Updated,Removed,Declined,NoChange} (+ String). ApplyOptions{Path,Target,Action,Yes,
    Out,Confirm}. ConfirmFunc = func(out io.Writer, path, diff string) bool. DefaultConfirm: non-TTY stdin
    auto-declines; --yes bypasses. Apply returns plain errors (the cmd layer wraps via exitcode.New).

- docfile: plan/005_c38aa48290f0/prd_snapshot.md
  why: §9.21 FR-I1 (surface + list columns: detected/config-path/status not-installed|installed|foreign),
       FR-I2 (detection gating: absent tool listed but install exits 1 with a note; git-alias needs only git),
       FR-I3 (the no-mangle protocol this surface drives — preview+confirm+backup+validate; for the docs
       summary), FR-I4/I5 (git-alias delegates to git config / lazygit YAML — context for the Entry asymmetry,
       NOT this subtask's impl), §15.3 (subcommand reference), §15.4 (exit codes: Error=1).
  section: "§9.21 (FR-I1/I2/I3), §15.3, §15.4"

- docfile: plan/005_c38aa48290f0/delta_prd.md
  why: R4 — the authoritative scope note: "explicit targets only (no install-everything default);
       detection-gated ... git-alias requires only git. v2.1 targets are exactly git-alias and lazygit
       (gitui is blocked upstream — FUTURE_SPEC.md)." Plus the §9.21 summary (no-mangle protocol is the core).
  section: "R4 — Tool integrations"

- docfile: plan/005_c38aa48290f0/architecture/external_deps.md
  why: §2 (no-mangle protocol IS the guarantee — for the docs summary), §7 (git alias mechanics — T2.S1's
       context, NOT this subtask), §1 (lazygit customCommands schema — T2.S2's context). Confirms the
       file-edit vs command-delegate asymmetry the Entry interface must accommodate.
  section: "§2, §7 (context only)"

- file: internal/provider/registry.go
  why: THE registry shape to mirror — NewRegistry(overrides), Get(name)(T,bool), List() []T sorted ascending
       by Name, IsInstalled via exec.LookPath. Construct-fresh (no global mutable state). My
       integrate.NewRegistry([]Entry) takes the slice as a constructor arg (pure, testable).
  pattern: "map[string]Entry internal; List() sorts by Name via sort.Slice; Get returns (Entry,bool)."
  gotcha: "provider.Registry seeds from BuiltinManifests(); integrate has NO built-in targets in S2 —
           NewRegistry([]Entry) seeds EXACTLY from the passed slice (nil ⇒ empty registry). Do not invent
           a built-in target list here (T2 owns git-alias/lazygit)."

- file: internal/cmd/providers.go
  why: THE closest cmd-layer precedent for a list command — providersCmd group (no RunE ⇒ bare prints help),
       init() registration on rootCmd (ZERO root.go edit), exitcode.New routing, cmd.OutOrStdout/
       ErrOrStderr, AND the pure-seam split: runProvidersList (RunE builds registry, calls print) vs
       printProvidersList(w, reg, default) (pure, tabwriter, takes io.Writer). MIRROR this exactly.
  pattern: "tabwriter.NewWriter(w,0,0,2,' ',0); header row 'NAME\\tDETECTED\\tDEFAULT'; ✓/✗; one row per
            List(); Flush(). Pure print fn takes (io.Writer, *Registry, ...). RunE wraps errors in exitcode.New."
  gotcha: "providers.go does NOT define its own PersistentPreRunE (it NEEDS cfg.Providers via Config()).
           integrate MUST define a no-op one (like hook) to SKIP config.Load — it does not need cfg."

- file: internal/cmd/hook.go
  why: THE precedent for (a) a command group that SKIPS config.Load via a no-op PersistentPreRunE
       (hookCmd.PersistentPreRunE = func(*cobra.Command,[]string) error { return nil }) — same rationale:
       edits user-owned files, works outside a repo, must not trigger FR-B3; (b) per-subcommand local flags
       (--print/--strict on hookInstallCmd) — the template for where T2's --alias-name/--key go; (c) the
       foreign-refusal / idempotent-status messaging style.
  pattern: "groupCmd with no-op PersistentPreRunE; leafCmd.RunE = runX; init(){ group.AddCommand(leaves...);
            rootCmd.AddCommand(group) }. Errors → exitcode.New(exitcode.Error, ...). Status verbs: Installed/
            Updated/No stagecoach X to remove."
  gotcha: "hook's --strict/--print are LOCAL to hookInstallCmd. integrate's --yes is shared by install AND
           remove → register it as a PERSISTENT flag on the integrateCmd parent (inherited by both; list
           harmlessly ignores it)."

- file: internal/cmd/root.go
  why: rootCmd (the registration target), shouldSkipConfigLoad (NOT extended — integrate uses the no-op
       PersistentPreRunE override, NOT the name-list in shouldSkipConfigLoad; that list is for config init/
       path/upgrade only), Config() accessor (integrate does NOT call it — it skips config.Load).
  pattern: "DO NOT edit root.go. Register via rootCmd.AddCommand in internal/cmd/integrate.go init()."
  gotcha: "shouldSkipConfigLoad is a SEPARATE mechanism (command-NAME-based, for config subcommands).
           integrate does NOT add itself there — the no-op PersistentPreRunE on integrateCmd is the override
           (cobra runs only the nearest PreRunE). Keep these two mechanisms distinct."

- file: internal/exitcode/exitcode.go
  why: exitcode.Success/Error + New(code,err) + ExitError + errors.As. Detection-gate / unknown-target /
       install-error → exitcode.New(exitcode.Error, err) (exit 1). Decline/NoChange → return nil (exit 0).
  pattern: "return exitcode.New(exitcode.Error, fmt.Errorf(\"stagecoach: %w\", err)). Never os.Exit."

- file: internal/cmd/providers_test.go  (and internal/cmd/hook_test.go)
  why: THE test style — saveRootState/restoreRootState (rootCmd out/err/RunE isolation), rootCmd.SetArgs +
       Execute(context.Background()) for wiring tests; per-command flag reset (resetHookFlags template →
       resetIntegrateFlags). providers_test also shows the substring/sort assertions for list output.
  pattern: "_,o,e,r := saveRootState(t); defer restoreRootState(t,nil,o,e,r); var out bytes.Buffer;
            rootCmd.SetOut(&out); rootCmd.SetErr(io.Discard); rootCmd.SetArgs([]string{...});
            Execute(context.Background()); assert on out.String()."
  gotcha: "Primary tests should NOT go through Execute — call the pure dispatch funcs
           (dispatchInstall/dispatchRemove/printIntegrateList) with a hand-built
           integrate.NewRegistry(fakes) and io.Buffers. Execute-level tests are for cobra WIRING only
           (empty list, unknown-target exit 1, --yes parses)."

- file: internal/ui/output.go
  why: ui.IsTerminal — already consumed by S1's DefaultConfirm (the non-TTY auto-decline gate). integrate's
       cmd layer does NOT import ui directly in S2 (it passes opts.Confirm=nil ⇒ DefaultConfirm, which uses
       IsTerminal). Listed for completeness; do NOT add a ui dependency to integrate.go.
```

### Current Codebase tree (relevant slice)

```bash
internal/
  integrate/                   # PACKAGE created by S1 (protocol.go); this subtask ADDS registry.go
    protocol.go                # S1 — Target/Apply/ApplyOptions/ApplyResult/Outcome/ConfirmFunc/
                               #        DefaultConfirm/BackupPath (CONSUME — do NOT edit)
    registry.go                # NEW (this subtask) — Status, Entry, InstallOptions/RemoveOptions,
                               #        InstallResult/RemoveResult, Registry, sentinels
  cmd/
    providers.go               # REF — list-command + pure-seam + tabwriter precedent (do NOT edit)
    hook.go                    # REF — no-op PersistentPreRunE (skip config.Load) + group/leaf/init (do NOT edit)
    root.go                    # REF — rootCmd registration target, shouldSkipConfigLoad (do NOT edit)
    integrate.go               # NEW (this subtask) — cobra group + leaves + defaultEntries seam + dispatch
    integrate_test.go          # NEW (this subtask) — fake Entry + dispatch matrix + wiring tests
    providers_test.go          # REF — saveRootState/Execute test style (do NOT edit)
    hook_test.go               # REF — resetFlags + round-trip test style (do NOT edit)
    root_test.go               # REF — shared helpers: loadEnvSetup/chdir/saveRootState/writeConfigFile/initRepo
  exitcode/exitcode.go         # REF — exitcode.New/Error/For (do NOT edit)
  provider/registry.go         # REF — Registry shape to mirror (do NOT edit)
  ui/output.go                 # REF — IsTerminal (consumed indirectly via S1's DefaultConfirm) (do NOT edit)
docs/
  cli.md                       # EDIT (Mode A) — add `integrate` group subsections under ## Subcommands
go.mod                         # UNCHANGED (no new require)
```

### Desired Codebase tree with files to be added and responsibility of file

```bash
internal/integrate/
  registry.go        # Status enum; Entry interface; InstallOptions/RemoveOptions; InstallResult/
                     #   RemoveResult; Registry (NewRegistry/Get/List); ErrUnknownTarget/ErrToolNotDetected.
                     #   Pure library (plain errors; cmd layer routes exit codes). NO cobra, NO os.Exit.
internal/cmd/
  integrate.go       # integrateCmd group (no-op PersistentPreRunE; persistent --yes) + list/install/
                     #   remove leaves; defaultEntries seam (nil in S2); runIntegrateList/Install/Remove
                     #   (RunE → build Registry → pure dispatch); printIntegrateList/dispatchInstall/
                     #   dispatchRemove (pure, take io.Writer + Registry). Registered via init() on rootCmd.
  integrate_test.go  # fake Entry (configurable detect/status/install + call log); dispatch matrix; wiring.
docs/
  cli.md             # + integrate list/install/remove subsections (Mode A) under ## Subcommands.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (integrate SKIPS config.Load): like hook, integrate edits user dotfiles (gitconfig, lazygit
// config) and works OUTSIDE a git repo. root's PersistentPreRunE runs config.Load (needs a repo dir for
// Layer 4 git-config) and triggers FR-B3's first-run bootstrap write. integrateCmd defines a NO-OP
// PersistentPreRunE (func(*cobra.Command,[]string) error { return nil }) — cobra runs only the NEAREST
// PreRunE, so this overrides root's. Do NOT add integrate to shouldSkipConfigLoad (that's a separate,
// command-NAME-based mechanism for config init/path/upgrade). Do NOT call Config().

// CRITICAL (reuse S1's Outcome — do NOT re-invent): InstallResult.Outcome and RemoveResult.Outcome are
// integrate.Outcome (Created/Updated/Removed/Declined/NoChange from S1's protocol.go). lazygit maps its
// ApplyResult.Outcome 1:1; git-alias maps its git-config result into the same set. The cmd layer prints
// based on Outcome — one reporting vocabulary across both target kinds.

// CRITICAL (git-alias does NOT use protocol.Apply): FR-I4 delegates the .gitconfig edit to `git config`.
// So the Entry abstraction must let git-alias own its OWN Install/Remove (running git config, with its own
// preview+confirm honoring Yes/Confirm), while lazygit's Install calls protocol.Apply. The Entry.Install
// method is the polymorphism point — do NOT assume all targets go through Apply.

// CRITICAL (Status is integrate's OWN enum, not hook's): integrate.Status{NotInstalled,Installed,Foreign}
// with String() "not installed"/"installed"/"foreign" (FR-I1 exact tokens). hook.Status{None,Stagecoach,
// Foreign} is a DIFFERENT type with different strings ("none"/"stagecoach (v1)"). Do NOT import or alias
// hook.Status. (A target whose tool is absent still has a Status — StatusNotInstalled — independent of
// DETECTED. DETECTED=✗ + STATUS=not-installed is the normal "not set up yet" row.)

// CRITICAL (DETECTED and STATUS are independent columns): DETECTED = is the TOOL on $PATH (Detect ctx);
// STATUS = is stagecoach's ENTRY present (Status ctx). A target can be DETECTED=✓ STATUS=not-installed
// (tool present, integration not yet applied) or DETECTED=✗ STATUS=installed (entry exists from a prior
// machine — list still shows it). list prints both; install gates on DETECTED only.

// GOTCHA (defaultEntries is the SINGLE registration seam): `var defaultEntries = func() []integrate.Entry
// { return nil }` in internal/cmd/integrate.go. S2 ships nil (empty registry). T2.S1 appends &gitAliasEntry,
// T2.S2 appends &lazygitEntry. Tests swap this var (or build NewRegistry(fakes) directly). Do NOT use an
// init()-based integrate.Register() with blank imports — that adds global mutable state the provider
// precedent avoids. The function-var form constructs fresh per call (mirrors provider.NewRegistry).

// GOTCHA (Confirm==nil ⇒ DefaultConfirm): prod passes opts.Confirm=nil; S1's DefaultConfirm gates on
// ui.IsTerminal(os.Stdin) and auto-declines non-TTY. Tests inject a fixed-bool Confirm (no real stdin/TTY).
// The lazygit Entry forwards Confirm to ApplyOptions.Confirm; git-alias uses it for its own alias preview.

// GOTCHA (detection gating is per-target, batch continues): install/remove loop: for each named target,
// Get (miss ⇒ unknown-target note + mark failed) → Detect (non-nil ⇒ note naming target/tool + mark
// failed) → else Install/Remove. CONTINUE with remaining targets even after a failure (best-effort). Exit
// 1 iff any target failed/unknown/gated. Decline/NoChange outcomes are NOT failures (exit 0). This is the
// batch semantics the work item implies ("install exits 1 with a note" — per target).

// GOTCHA (cobra.MinimumNArgs(1) for explicit targets): install/remove require ≥1 target (no "install
// everything" default — delta_prd.md R4). cobra.MinimumNArgs(1) enforces this at the arg layer (prints
// usage + exit 1 on zero args). list is cobra.NoArgs.

// GOTCHA (registry.go is pure library): return PLAIN errors (fmt.Errorf %w / sentinel vars). NO os.Exit,
// NO cobra import, NO exitcode import in registry.go. The cmd layer (integrate.go) wraps via exitcode.New.
// Mirrors S1's protocol.go discipline and provider/registry.go (which imports only stdlib + go-toml).

// GOTCHA (List sorting must be deterministic): Registry.List() sorts ascending by Entry.Name() via
// sort.Slice, exactly like provider.Registry.List(). `integrate list` output is then deterministic (the
// table order is stable regardless of registration order). Tests assert on substrings + ordering.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/integrate/registry.go  (NEW — package integrate; siblings: protocol.go from S1)
package integrate // import "github.com/dustin/stagecoach/internal/integrate"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
)

// Status is the integration state of one target (PRD §9.21 FR-I1). DISTINCT from hook.Status — integrate
// owns its own enum and exact report tokens.
type Status int

const (
	StatusNotInstalled Status = iota // no stagecoach-managed entry in the target's config
	StatusInstalled                  // stagecoach entry present (marker/key ours)
	StatusForeign                    // a conflicting entry exists at the target's key/alias
)

// String renders the FR-I1 tokens EXACTLY: "not installed" / "installed" / "foreign".
func (s Status) String() string {
	switch s {
	case StatusInstalled:
		return "installed"
	case StatusForeign:
		return "foreign"
	default:
		return "not installed"
	}
}

// Entry is one integration target (git-alias, lazygit, future). The cmd layer (list/install/remove) drives
// every target uniformly through this interface; each target owns its install/remove MECHANICS — lazygit
// calls protocol.Apply (FR-I5), git-alias delegates to `git config` (FR-I4, which does NOT use Apply). The
// four registry-facing methods (Name/Detect/ConfigPath/Status) back `list` + detection gating; Install/
// Remove back the commands. T2.S1/T2.S2 implement this; S2 tests with fakes.
type Entry interface {
	// Name is the target's CLI token (e.g. "git-alias", "lazygit") — the <target> argument. Stable, unique.
	Name() string

	// Detect reports whether the target's TOOL is on $PATH (FR-I2 detection gating). nil ⇒ present (install
	// may proceed); non-nil ⇒ absent — the command prints the error's message as the note and skips install
	// for this target (exit 1). git-alias detects git (always present for stagecoach); lazygit detects lazygit.
	Detect(ctx context.Context) error

	// ConfigPath resolves the config file/path the target edits (FR-I1 "resolved config path" column + the
	// note/error context). May be empty/"—" if the target cannot resolve it (e.g. tool absent) — never fatal
	// for `list`; it just shows "—".
	ConfigPath(ctx context.Context) (string, error)

	// Status reports the integration's current state (FR-I1). Reads the target's config for the stagecoach
	// entry: NotInstalled (absent) / Installed (ours) / Foreign (a conflicting entry). Independent of Detect
	// (a target can be StatusInstalled with the tool since uninstalled).
	Status(ctx context.Context) (Status, error)

	// Install applies the integration. The target decides HOW (lazygit: protocol.Apply; git-alias: git
	// config). opts carries the shared controls every target honors: Yes (skip confirm, --yes), Out (preview/
	// status writer), Confirm (nil ⇒ integrate.DefaultConfirm). Returns an InstallResult (Outcome is S1's
	// unified enum) and a plain error on failure (the cmd layer maps to exit 1). Decline/NoChange are
	// reported via Outcome, NOT as errors.
	Install(ctx context.Context, opts InstallOptions) (InstallResult, error)

	// Remove deletes the stagecoach entry (FR-I6 uninstall symmetry). Same controls/contract as Install.
	Remove(ctx context.Context, opts RemoveOptions) (RemoveResult, error)
}

// InstallOptions are the shared controls passed to Entry.Install (PRD §9.21 FR-I3c — preview+confirm).
// Target-specific values (--alias-name, --key) are NOT here — T2's defaultEntries() constructs each Entry
// with its resolved flag values, so the interface stays narrow. Confirm==nil ⇒ DefaultConfirm (S1).
type InstallOptions struct {
	Yes     bool        // --yes: skip the y/N confirm (scripts) — lazygit → ApplyOptions.Yes; git-alias → own confirm
	Out     io.Writer   // preview diff + status (cmd's stderr in prod); nil ⇒ os.Stderr (target decides)
	Confirm ConfirmFunc // y/N prompt; nil ⇒ DefaultConfirm (TTY-gated, S1). Reused from protocol.go.
}

// RemoveOptions mirror InstallOptions for Entry.Remove (remove runs the same preview+confirm protocol, FR-I3).
type RemoveOptions struct {
	Yes     bool
	Out     io.Writer
	Confirm ConfirmFunc
}

// InstallResult is what Entry.Install did, for the cmd layer's per-target status line. Outcome is S1's
// integrate.Outcome (Created/Updated/Removed/Declined/NoChange) — the unified vocabulary across target kinds.
type InstallResult struct {
	Outcome Outcome // S1's enum — lazygit copies ApplyResult.Outcome; git-alias maps its own result
	Target  string  // the target name (echoed for the status line)
	Path    string  // the config path touched (lazygit: ApplyResult.Path; git-alias: the .gitconfig)
	Backup  string  // the backup path written (lazygit: ApplyResult.Backup; git-alias: "" — git owns the file)
}

// RemoveResult mirrors InstallResult for Entry.Remove.
type RemoveResult struct {
	Outcome Outcome
	Target  string
	Path    string
	Backup  string
}

// Sentinels for the dispatch refusal paths. Callers use errors.Is.
var (
	// ErrUnknownTarget is returned/wrapped when a named target is not in the registry. The cmd layer prints
	// "unknown target <x>; see `stagecoach integrate list`" and exits 1.
	ErrUnknownTarget = errors.New("unknown integration target")
	// ErrToolNotDetected wraps a target-specific Detect failure for detection-gating context (FR-I2). The
	// target's Detect error is the primary signal; this is the category the cmd layer recognizes.
	ErrToolNotDetected = errors.New("target tool not detected on $PATH")
)

// Registry holds the compiled-in integration targets (PRD §9.21). Mirrors provider.Registry: a name-keyed
// map, List() sorted ascending by Name (deterministic for `list`), Get for unknown-target refusal. Seeds
// EXACTLY from the passed slice (no built-in targets in S2 — T2 supplies git-alias/lazygit). Pure data
// structure; no exec inside (Entry methods do the probing). Construct fresh per command (no global state).
type Registry struct {
	entries map[string]Entry
}

// NewRegistry builds a Registry from entries. Duplicate Names ⇒ the last wins (defensive; T2's list has
// none). nil/empty ⇒ an empty registry (S2's shipped state; `list` shows a header-only table).
func NewRegistry(entries []Entry) *Registry {
	m := make(map[string]Entry, len(entries))
	for _, e := range entries {
		if e != nil {
			m[e.Name()] = e
		}
	}
	return &Registry{entries: m}
}

// Get returns the Entry for name and whether it exists. Unknown-name refusal (FR-I1/I2 dispatch) uses this.
func (r *Registry) Get(name string) (Entry, bool) {
	e, ok := r.entries[name]
	return e, ok
}

// List returns every Entry, sorted ascending by Name (deterministic for `integrate list`). Fresh slice.
func (r *Registry) List() []Entry {
	out := make([]Entry, 0, len(r.entries))
	for _, e := range r.entries {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// --- the cmd layer (internal/cmd/integrate.go) ---

// defaultEntries is the SINGLE registration seam (PRD §9.21). S2 ships NONE — the command surface +
// registry + detection gating are provably correct with fakes before any concrete target exists. T2.S1
// (git-alias) and T2.S2 (lazygit) APPEND their integrate.Entry impls here (one line each). A function-var
// (not init/Register) so the Registry is always freshly built (mirrors provider.NewRegistry's discipline)
// and tests can swap it.
var defaultEntries = func() []integrate.Entry {
	return nil // T2.S1: append &gitAliasEntry{...}; T2.S2: append &lazygitEntry{...}
}

var flagIntegrateYes bool // --yes (persistent on integrateCmd; install+remove honor it, list ignores it)

var integrateCmd = &cobra.Command{
	Use:   "integrate",
	Short: "Wire stagecoach into installed git tools",
	Long: `Install or remove stagecoach integrations for the git tools you already run (PRD §9.21).

Targets are explicit — name one or more of the supported tools (see 'stagecoach integrate list').
Every file edit runs the no-mangle protocol: a unified-diff preview, a y/N prompt (use --yes to skip),
a timestamped backup, and a post-write re-parse with automatic restore on failure.`,
	SilenceErrors:     true,
	SilenceUsage:      true,
	PersistentPreRunE: func(*cobra.Command, []string) error { return nil }, // SKIP config.Load (like hook)
}

var integrateListCmd = &cobra.Command{
	Use: "list", Short: "List integration targets and their status",
	Args: cobra.NoArgs, SilenceErrors: true, SilenceUsage: true, RunE: runIntegrateList,
}
var integrateInstallCmd = &cobra.Command{
	Use: "install <target>…", Short: "Install one or more stagecoach integrations",
	Args: cobra.MinimumNArgs(1), SilenceErrors: true, SilenceUsage: true, RunE: runIntegrateInstall,
}
var integrateRemoveCmd = &cobra.Command{
	Use: "remove <target>…", Short: "Remove one or more stagecoach integrations",
	Args: cobra.MinimumNArgs(1), SilenceErrors: true, SilenceUsage: true, RunE: runIntegrateRemove,
}

func init() {
	integrateCmd.PersistentFlags().BoolVar(&flagIntegrateYes, "yes", false,
		"Skip the preview prompt and apply changes directly (for scripts)")
	integrateCmd.AddCommand(integrateListCmd, integrateInstallCmd, integrateRemoveCmd)
	rootCmd.AddCommand(integrateCmd) // NO edit to root.go (providers.go/hook.go pattern)
}

// runIntegrateList — RunE: build the registry, delegate to the pure printer.
func runIntegrateList(cmd *cobra.Command, _ []string) error {
	reg := integrate.NewRegistry(defaultEntries())
	printIntegrateList(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), reg)
	return nil
}

// printIntegrateList — PURE: the FR-I1 table (TARGET/DETECTED/STATUS/CONFIG), one row per List() (sorted).
func printIntegrateList(ctx context.Context, stdout, stderr io.Writer, reg *integrate.Registry) {
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TARGET\tDETECTED\tSTATUS\tCONFIG")
	for _, e := range reg.List() {
		detected := "✗"
		if err := e.Detect(ctx); err == nil {
			detected = "✓"
		}
		status := "not installed"
		if s, err := e.Status(ctx); err == nil {
			status = s.String()
		}
		cfg := "—"
		if p, err := e.ConfigPath(ctx); err == nil && p != "" {
			cfg = p
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", e.Name(), detected, status, cfg)
	}
	tw.Flush()
}

// dispatchInstall — PURE: per-target Detect gate → Install; batch continue-on-failure; exit 1 iff any
// failed/unknown/gated. Decline/NoChange are NOT failures. Returns an exitcode-wrapped error on failure.
func dispatchInstall(ctx context.Context, reg *integrate.Registry, targets []string, opts integrate.InstallOptions, stdout, stderr io.Writer) error {
	failed := false
	for _, name := range targets {
		e, ok := reg.Get(name)
		if !ok {
			fmt.Fprintf(stderr, "stagecoach: unknown target %q; see `stagecoach integrate list`.\n", name)
			failed = true
			continue
		}
		if err := e.Detect(ctx); err != nil { // FR-I2 detection gating
			fmt.Fprintf(stderr, "stagecoach: %s requires its tool on $PATH, which was not detected (%v); skipping.\n", name, err)
			failed = true
			continue
		}
		res, err := e.Install(ctx, opts)
		if err != nil {
			fmt.Fprintf(stderr, "stagecoach: install %s failed: %v\n", name, err)
			failed = true
			continue
		}
		fmt.Fprintln(stdout, formatInstallResult(res)) // "Installed/Updated/Created/No change/Declined ..."
	}
	if failed {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: one or more targets failed"))
	}
	return nil
}

// runIntegrateInstall — RunE: build registry + opts, delegate to pure dispatch.
func runIntegrateInstall(cmd *cobra.Command, args []string) error {
	reg := integrate.NewRegistry(defaultEntries())
	opts := integrate.InstallOptions{Yes: flagIntegrateYes, Out: cmd.ErrOrStderr(), Confirm: nil /*⇒Default*/}
	return dispatchInstall(cmd.Context(), reg, args, opts, cmd.OutOrStdout(), cmd.ErrOrStderr())
}
// (runIntegrateRemove + dispatchRemove are symmetric: RemoveOptions + RemoveResult + "Removed"/"No change"/"Declined".)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/integrate/registry.go — Status + Entry + options/results + sentinels
  - IMPLEMENT the data-models block's TYPES: Status enum (NotInstalled/Installed/Foreign) + String()
    (exact tokens "not installed"/"installed"/"foreign"); Entry interface (Name/Detect/ConfigPath/Status/
    Install/Remove — see doc comments); InstallOptions/RemoveOptions {Yes bool; Out io.Writer; Confirm
    integrate.ConfirmFunc}; InstallResult/RemoveResult {Outcome integrate.Outcome; Target,Path,Backup string};
    ErrUnknownTarget/ErrToolNotDetected sentinels.
  - GOTCHA: Outcome/ConfirmFunc are S1's — reference integrate.Outcome / integrate.ConfirmFunc (same package,
    same file is fine; protocol.go is a sibling). Do NOT re-declare them.
  - GOTCHA: Status is integrate's OWN enum — do NOT import/alias hook.Status.
  - PLACEMENT: internal/integrate/registry.go (sibling to S1's protocol.go).

Task 2: CREATE internal/integrate/registry.go — Registry
  - IMPLEMENT Registry{entries map[string]Entry}; NewRegistry([]Entry) (nil ⇒ empty; nil-elements skipped;
    dup-Name ⇒ last wins); Get(name)(Entry,bool); List() []Entry sorted ascending by Name (sort.Slice).
  - FOLLOW pattern: internal/provider/registry.go (NewRegistry seeds from arg; List sorts; Get bool).
  - GOTCHA: NO built-in target seeding (S2 ships empty); NO exec inside Registry (Entry methods probe).
    Pure data structure (stdlib imports only: context, errors, io, sort).

Task 3: CREATE internal/cmd/integrate.go — cobra group + leaves + flags + init
  - IMPLEMENT integrateCmd (Use/Short/Long; SilenceErrors/Usage; NO RunE ⇒ bare prints help; no-op
    PersistentPreRunE to SKIP config.Load — hook.go precedent); integrateListCmd (cobra.NoArgs),
    integrateInstallCmd/integrateRemoveCmd (cobra.MinimumNArgs(1)); flagIntegrateYes (--yes PERSISTENT on
    integrateCmd); init() → AddCommand(leaves) + rootCmd.AddCommand(integrateCmd).
  - GOTCHA: --yes is PERSISTENT on the parent (install+remove inherit; list ignores). Do NOT add integrate
    to root.go's shouldSkipConfigLoad (separate mechanism). Do NOT edit root.go.
  - FOLLOW pattern: internal/cmd/hook.go (group/leaf/init/PersistentPreRunE) + internal/cmd/providers.go
    (exitcode.New routing, OutOrStdout/ErrOrStderr, SilenceErrors/Usage).

Task 4: CREATE internal/cmd/integrate.go — defaultEntries seam + runE + pure dispatch
  - IMPLEMENT `var defaultEntries = func() []integrate.Entry { return nil }` (documented seam); runIntegrateList/
    runIntegrateInstall/runIntegrateRemove (build integrate.NewRegistry(defaultEntries()), build opts from
    flags — InstallOptions{Yes: flagIntegrateYes, Out: cmd.ErrOrStderr(), Confirm: nil}, delegate to pure
    dispatch); printIntegrateList (tabwriter TARGET/DETECTED/STATUS/CONFIG; ✓/✗ via Detect; Status().String();
    ConfigPath or "—"); dispatchInstall/dispatchRemove (per-target Get→unknown-note; Detect→gate-note;
    Install/Remove→format+print; batch continue; exitcode.Error iff any failed; Decline/NoChange not failures).
  - FOLLOW pattern: providers.go runProvidersList→printProvidersList pure seam; hook.go exitcode messaging.
  - GOTCHA: DETECTED and STATUS are independent columns (see Known Gotchas). formatInstallResult maps
    Outcome→verb (Created/Updated→"Installed/Updated", NoChange→"No changes for", Declined→"Declined").

Task 5: CREATE internal/cmd/integrate_test.go — fake Entry + dispatch matrix
  - DEFINE fakeEntry (in-package, unexported): fields name, detectErr, status, statusErr, configPath,
    configErr, installRes, installErr, removeRes, removeErr; a call log (e.g. installCalled bool) for
    assertions. All Entry methods implemented; ctx accepted/ignored.
  - TESTS (call pure dispatch with integrate.NewRegistry([]Entry{fakes}) + bytes.Buffers):
    * TestDispatchInstall_DetectionGateExit1: a target with detectErr≠nil → note to stderr names target;
      failed; returns exitcode.Error. The OTHER named target (detectErr=nil) is still Installed (batch
      continue) — assert both its stdout line AND the first's stderr note appear; exit code 1.
    * TestDispatchInstall_UnknownTarget: name not in registry → "unknown target ... see `integrate list`";
      exit 1.
    * TestDispatchInstall_AllInstalled: all targets detect+install OK → stdout has each result; exit 0 (nil).
    * TestDispatchInstall_DeclineAndNoChangeNotErrors: targets whose InstallResult.Outcome is Declined /
      NoChange → printed but NOT a failure → exit 0.
    * TestDispatchInstall_InstallError: installErr≠nil → "install X failed" note; exit 1.
    * TestDispatchRemove_*: symmetric (RemoveOptions; "Removed"/"No changes for"/"Declined"; detection gate;
      unknown).
    * TestPrintIntegrateList_Table: a registry of 3 fakes (detected/undetected × installed/not/foreign ×
      config path/empty) → header + 3 rows sorted by Name; DETECTED ✓/✗; STATUS exact tokens; CONFIG path
      or "—". Assert substrings + ascending order.
    * TestPrintIntegrateList_Empty: nil-entries registry → header row only (no data rows); exit-free.
    * TestRegistry_GetList: NewRegistry(fakes).Get hit/miss; List() sorted by Name; nil entries skipped.
    * TestStatus_String: the three tokens.
  - WIRING tests (saveRootState/restoreRootState + resetIntegrateFlags + SetArgs + Execute):
    * TestIntegrateList_EmptyWiring: `integrate list` (defaultEntries=nil) → stdout header-only; exit 0.
    * TestIntegrateInstall_BogusExit1: `integrate install bogus` → exit 1; stderr has "unknown target".
    * TestIntegrateInstall_NoArgsUsage: `integrate install` (0 args) → cobra usage error / exit 1.
    * TestIntegrateYesFlag: `integrate install <fake> --yes` with defaultEntries swapped to a fake recording
      opts.Yes → assert the fake saw Yes=true (use the defaultEntries-var swap, restored in defer).
  - FOLLOW pattern: providers_test.go (saveRootState/SetArgs/Execute + substring/sort asserts), hook_test.go
    (resetFlags template → resetIntegrateFlags resets flagIntegrateYes + the --yes Changed bit).

Task 6: EDIT docs/cli.md — [Mode A] integrate group (under ## Subcommands, before ## Exit codes)
  - ADD three subsections: `### `integrate list`` (table: TARGET/DETECTED/STATUS/CONFIG; targets are
    git-alias + lazygit; gitui blocked upstream → FUTURE_SPEC.md), `### `integrate install <target>…``
    (explicit targets, ≥1; --yes; detection gating: absent tool listed but install exits 1 with a note;
    git-alias needs only git), `### `integrate remove <target>…`` (uninstall symmetry). A short "No-mangle
    protocol" callout: every file edit shows a unified diff, asks y/N, writes a timestamped backup, and
    re-validates after writing with automatic restore on failure (FR-I3) — cross-ref §9.21.
  - FOLLOW the existing per-command heading + example-block style (see hook/providers subsections L62–156).

Task 7: VERIFY build/test/lint (no go.mod change)
  - go build ./... ; go test ./internal/integrate/... ./internal/cmd/... -v ; go test ./... ;
    go vet ./... ; golangci-lint run ; gofmt -l internal/integrate internal/cmd. Confirm go.mod UNCHANGED
    (git diff go.mod empty — yaml.v3 is T2.S2's, NOT this subtask's).
```

### Implementation Patterns & Key Details

```go
// formatInstallResult maps S1's Outcome to the per-target stdout verb (mirrors hook's Installed/Updated
// messaging). Decline/NoChange are informational, NOT errors (exit 0).
func formatInstallResult(r integrate.InstallResult) string {
	switch r.Outcome {
	case integrate.OutcomeCreated:
		return fmt.Sprintf("Installed %s integration (created %s).", r.Target, r.Path)
	case integrate.OutcomeUpdated:
		s := fmt.Sprintf("Updated %s integration (%s).", r.Target, r.Path)
		if r.Backup != "" {
			s += fmt.Sprintf(" Backup: %s", r.Backup)
		}
		return s
	case integrate.OutcomeRemoved:
		return fmt.Sprintf("Removed %s integration.", r.Target)
	case integrate.OutcomeDeclined:
		return fmt.Sprintf("Declined %s — nothing was changed.", r.Target)
	default: // OutcomeNoChange
		return fmt.Sprintf("No changes for %s (already installed).", r.Target)
	}
}

// resetIntegrateFlags — the per-command flag reset (hook_test.go's resetHookFlags template), for the
// Execute-level wiring tests. restoreRootState resets rootCmd's persistent flags but NOT integrateCmd's
// local persistent --yes; reset it + its Changed bit between tests.
func resetIntegrateFlags(t *testing.T) {
	t.Helper()
	flagIntegrateYes = false
	if f := integrateCmd.PersistentFlags().Lookup("yes"); f != nil && f.Changed {
		f.Changed = false
	}
}

// The fakeEntry test vehicle — proves the surface before any real target exists. This is the SHAPE every
// real Entry (gitAliasEntry/lazygitEntry) takes; it exercises detection gating + status reporting + batch
// dispatch without git/lazygit/yaml.
type fakeEntry struct {
	name       string
	detectErr  error         // nil ⇒ DETECTED=✓ and install proceeds; non-nil ⇒ detection gate
	status     integrate.Status
	statusErr  error
	configPath string
	configErr  error
	installRes integrate.InstallResult
	installErr error
	removeRes  integrate.RemoveResult
	removeErr  error
	installCalled bool
	installOpts   integrate.InstallOptions // records Yes/Out/Confirm for the --yes test
}
func (f *fakeEntry) Name() string                                         { return f.name }
func (f *fakeEntry) Detect(context.Context) error                         { return f.detectErr }
func (f *fakeEntry) ConfigPath(context.Context) (string, error)           { return f.configPath, f.configErr }
func (f *fakeEntry) Status(context.Context) (integrate.Status, error)     { return f.status, f.statusErr }
func (f *fakeEntry) Install(_ context.Context, o integrate.InstallOptions) (integrate.InstallResult, error) {
	f.installCalled = true; f.installOpts = o; return f.installRes, f.installErr
}
func (f *fakeEntry) Remove(context.Context, integrate.RemoveOptions) (integrate.RemoveResult, error) {
	return f.removeRes, f.removeErr
}
```

### Integration Points

```yaml
PROVIDES (to P1.M4.T2.S1 and P1.M4.T2.S2 — the consumers):
  - integrate.Entry                # the interface every target implements (Name/Detect/ConfigPath/Status/Install/Remove)
  - integrate.Registry             # NewRegistry([]Entry)/Get/List — the dispatch table
  - integrate.Status               # NotInstalled/Installed/Foreign (+ String)
  - integrate.InstallOptions       # {Yes, Out, Confirm} — shared controls T2's Install receives
  - integrate.RemoveOptions        # symmetric
  - integrate.InstallResult        # {Outcome, Target, Path, Backup} — T2 returns this (Outcome is S1's)
  - integrate.RemoveResult         # symmetric
  - integrate.ErrUnknownTarget     # sentinel (optional use by targets that want to surface it)
  - integrate.ErrToolNotDetected   # sentinel (category for Detect failures)
  - cmd.defaultEntries             # THE registration seam — T2 appends &gitAliasEntry{...} / &lazygitEntry{...}
  - cmd.dispatchInstall/Remove     # pure dispatch (T2 does NOT call these — they call the target's Install/Remove)
  - cmd.printIntegrateList         # pure table renderer

CONSUMES (do NOT re-implement):
  - integrate.Target / Apply / ApplyOptions / ApplyResult / Outcome / ConfirmFunc / DefaultConfirm /
    BackupPath (all from S1's protocol.go — same package).
  - internal/exitcode (cmd layer: exitcode.New(exitcode.Error, err)).
  - the cobra + tabwriter + init()-on-rootCmd conventions (providers.go/hook.go).

OUT OF SCOPE (owned by sibling subtasks — do NOT implement):
  - S1 (parallel): the protocol engine itself (Target/Apply/Outcome) — CONSUME only.
  - T2.S1: the gitAliasEntry Entry impl + the --alias-name flag + its line in defaultEntries. Delegates the
           edit to `git config --global alias.<name> '!stagecoach'` (FR-I4; external_deps.md §7).
  - T2.S2: the lazygitEntry Entry impl (yaml.v3 Node API, comment-preserving) + the --key flag + its line in
           defaultEntries + adds gopkg.in/yaml.v3 to go.mod (FR-I5; external_deps.md §1/§2).
  - Any concrete Entry, the real git-config / lazygit-config editing, yaml.v3.
  - P1.M7 (docs coherence sweep) owns README.md; this subtask edits ONLY docs/cli.md.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach-competitor-feature-parity
gofmt -w internal/integrate/registry.go internal/cmd/integrate.go internal/cmd/integrate_test.go
go build ./...            # the new package + cmd file must compile (integrate pkg now has registry.go + S1's protocol.go)
go vet ./...
golangci-lint run
# go.mod MUST be unchanged (no new require — yaml.v3 is T2.S2's addition):
git diff --name-only go.mod && echo "WARN: go.mod touched" || echo "go.mod clean OK"
# Expected: zero errors; go.mod clean.
```

### Level 2: Unit Tests (Component Validation)

```bash
go test ./internal/integrate/... -v
# Expected: S1's protocol tests still pass (unchanged) + new registry tests (TestRegistry_GetList, TestStatus_String).
go test ./internal/cmd/... -run Integrate -v
# Expected: all pass — DispatchInstall_DetectionGateExit1, DispatchInstall_UnknownTarget,
#   DispatchInstall_AllInstalled, DispatchInstall_DeclineAndNoChangeNotErrors, DispatchInstall_InstallError,
#   DispatchRemove_*, PrintIntegrateList_Table, PrintIntegrateList_Empty, IntegrateList_EmptyWiring,
#   IntegrateInstall_BogusExit1, IntegrateInstall_NoArgsUsage, IntegrateYesFlag.
go test ./...     # full suite — confirm no regression in providers/hook/config/default_action
# Expected: all pass.
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach
# list — empty registry (S2 ships no concrete target): header-only table, exit 0
/tmp/stagecoach integrate list
# Expected: prints "TARGET  DETECTED  STATUS  CONFIG" header, no data rows (T2 populates them), exit 0.

# unknown target — exit 1 + discoverable note
/tmp/stagecoach integrate install bogus; echo "exit=$?"
# Expected: stderr "stagecoach: unknown target \"bogus\"; see `stagecoach integrate list`.", exit=1.

# explicit-targets arg enforcement
/tmp/stagecoach integrate install; echo "exit=$?"
# Expected: cobra usage error (MinimumNArgs(1)), exit=1.

# --help surfaces all three leaves + the --yes flag
/tmp/stagecoach integrate --help
# Expected: Available Commands: install, list, remove; Flags: --yes.

# works OUTSIDE a git repo (confirms the no-op PersistentPreRunE skipped config.Load / no FR-B3 bootstrap)
cd /tmp && /tmp/stagecoach integrate list; echo "exit=$?"
# Expected: header-only table, exit 0 (no "config not found" / no bootstrap write).
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Detection-gating + batch-continue behavior, end-to-end via the dispatch func with fakes (the canonical
# proof the surface is correct before T2 exists). Re-run with -count=1 to defeat the test cache:
go test ./internal/cmd/... -run 'DispatchInstall_DetectionGate|DispatchInstall_AllInstalled' -v -count=1
# Status-reporting fidelity (the three FR-I1 tokens in the table):
go test ./internal/cmd/... -run 'PrintIntegrateList_Table|TestStatus_String' -v -count=1
# --yes flows through to the target's InstallOptions:
go test ./internal/cmd/... -run IntegrateYesFlag -v -count=1
# Expected: each passes deterministically (no real git/lazygit/yaml/TTY involved — pure fakes).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...`; `go test ./...`; `go vet ./...`; `golangci-lint run`; `gofmt -l` clean.
- [ ] `go.mod` UNCHANGED (no new `require` — yaml.v3 is T2.S2's; git-alias delegates to `git config`).
- [ ] `internal/integrate/registry.go` imports ONLY stdlib (no cobra, no exitcode, no os.Exit) — pure library.
- [ ] `internal/cmd/integrate.go` registers via `init()` on `rootCmd` — ZERO edit to root.go.
- [ ] `integrateCmd` has a no-op `PersistentPreRunE` (skips config.Load; works outside a repo; no FR-B3 bootstrap).

### Feature Validation
- [ ] `list` prints TARGET/DETECTED/STATUS/CONFIG (sorted by Name); empty registry ⇒ header only; exit 0.
- [ ] DETECTED (✓/✗ via Detect) and STATUS (NotInstalled/Installed/Foreign via Status) are INDEPENDENT columns.
- [ ] `install`/`remove` take ≥1 explicit targets (cobra.MinimumNArgs(1)); no "install everything" default.
- [ ] Detection gating (FR-I2): absent-tool target ⇒ stderr note + skip; remaining targets still attempted; exit 1 iff any failed.
- [ ] Unknown target ⇒ "unknown target <x>; see `stagecoach integrate list`" + exit 1.
- [ ] `--yes` (persistent on parent) flows into InstallOptions.Yes / RemoveOptions.Yes; list ignores it.
- [ ] Decline/NoChange outcomes are reported (stdout) but NOT failures (exit 0).
- [ ] `Entry`/`Registry`/`Status`/`InstallOptions`/`InstallResult`/sentinels are the exact contract T2 implements.

### Code Quality Validation
- [ ] Mirrors provider.Registry (construct-fresh, sorted List, Get bool) and providers.go/hook.go (cobra + exitcode).
- [ ] Pure dispatch (`dispatchInstall`/`dispatchRemove`/`printIntegrateList`) takes `io.Writer` + `*Registry` — testable without cobra.
- [ ] `defaultEntries` is a function-var (fresh per call; swappable in tests) — no global mutable registry state.
- [ ] Reuses S1's `Outcome`/`ConfirmFunc`/`DefaultConfirm` — does NOT re-invent an outcome enum or confirm prompt.
- [ ] `integrate.Status` is its own enum (distinct from `hook.Status`); exact FR-I1 tokens.

### Documentation & Deployment
- [ ] docs/cli.md has `integrate list`/`install`/`remove` subsections (Mode A) under `## Subcommands`.
- [ ] Docs state: targets = git-alias + lazygit (gitui blocked → FUTURE_SPEC.md); detection gating; no-mangle protocol summary.
- [ ] No README.md edit (P1.M7 owns the coherence sweep).

---

## Anti-Patterns to Avoid

- ❌ Don't implement any concrete `Entry` (git-alias/lazygit), add yaml.v3, or do the real git-config/lazygit-config editing — those are T2.S1/T2.S2. S2 ships an EMPTY registry + fakes only.
- ❌ Don't re-invent an outcome enum or a confirm prompt — reuse S1's `integrate.Outcome` / `ConfirmFunc` / `DefaultConfirm`. InstallResult.Outcome IS integrate.Outcome.
- ❌ Don't alias or import `hook.Status` — integrate has its own `Status` (NotInstalled/Installed/Foreign) with its own exact strings ("not installed"/"installed"/"foreign").
- ❌ Don't forget the no-op `PersistentPreRunE` on `integrateCmd` — without it, root's config.Load runs (needs a repo, triggers FR-B3 bootstrap), breaking `integrate list` outside a repo. This is the hook.go precedent.
- ❌ Don't add integrate to `shouldSkipConfigLoad` in root.go — that's a separate NAME-based mechanism for config init/path/upgrade; the no-op PersistentPreRunE is the override. Don't conflate them. Don't edit root.go.
- ❌ Don't use `init()`-based `integrate.Register()` + blank imports — global mutable state the provider precedent avoids. Use the `defaultEntries` function-var seam (construct-fresh, test-swappable).
- ❌ Don't make DETECTED and STATUS the same column — they're independent (tool-on-$PATH vs entry-present). A target can be detected-but-not-installed or installed-but-tool-gone.
- ❌ Don't fail-fast on the first bad target in a batch — continue with the rest (best-effort), then exit 1 iff any failed. The user naming `install git-alias lazygit` expects git-alias to land even if lazygit is absent.
- ❌ Don't treat Decline/NoChange as errors — they're reported on stdout and the command exits 0 (only detection-gate / unknown-target / install-error ⇒ exit 1).
- ❌ Don't call `os.Exit` in registry.go or import cobra/exitcode there — it's pure library code returning plain errors; the cmd layer wraps via `exitcode.New`.
- ❌ Don't write Execute-level tests for the dispatch logic — call the pure `dispatchInstall`/`dispatchRemove`/`printIntegrateList` with a hand-built `NewRegistry(fakes)` + `bytes.Buffer`. Reserve Execute tests for cobra WIRING (empty list, unknown-target exit 1, --yes parses, no-args usage).

---

## Confidence Score

**9/10** — one-pass success likelihood is high. The contract is precisely specified: the exact `Entry`
method set + `InstallOptions`/`Result` shapes (reusing S1's `Outcome`/`ConfirmFunc`/`DefaultConfirm`), the
exact `Registry` API (mirroring the in-repo `provider.Registry`), the exact cobra wiring (parent no-op
`PersistentPreRunE` — the in-repo hook.go precedent; `init()` registration; `cobra.MinimumNArgs(1)`), the
exact detection-gating + batch-dispatch semantics (per-target Detect gate, continue-on-failure, exit 1 iff
any failed, Decline/NoChange exit 0), the `defaultEntries()` registration seam for T2, the tabwriter table
layout (the in-repo providers.go `printProvidersList` precedent), the fake-Entry test vehicle + full matrix,
and the docs/cli.md placement + content. The residual uncertainty is cosmetic: the exact wording of the
per-target status lines (`formatInstallResult`) and the DETECTED note phrasing — both are test-asserted on
stable substrings, so a wording tweak is a one-line edit, not a redesign. No new dependency, no concrete
target, no config.Load — the scope is tight and both consumers (T2.S1/T2.S2) and the upstream contract
(S1) are named explicitly.
