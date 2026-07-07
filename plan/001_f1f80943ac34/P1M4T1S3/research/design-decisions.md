# P1.M4.T1.S3 — providers list/show: Design Decisions

Work item: add a `providers` command group with `list` and `show <name>` subcommands to the
Stagecoach CLI. INPUT = the `provider.Registry` built in P1.M2.T3.S1. OUTPUT = working
`stagecoach providers list` and `stagecoach providers show <name>`.

This file records the 12 decisions that govern the PRP. Each is justified against the on-disk
contracts (registry.go, root.go, manifest.go, config.go, exitcode.go, pkg/stagecoach.buildDeps).

---

## §0 — File plan: 2 NEW files, ZERO edits to root.go (registration via init())

**Decision.** Create `internal/cmd/providers.go` + `internal/cmd/providers_test.go`. Register the
`providers` command group with `rootCmd.AddCommand(providersCmd)` inside an `init()` in
`providers.go`. Do NOT edit `root.go`.

**Why.** (1) Go allows multiple `init()` functions in one package; `providers.go`'s `init()` runs
alongside `root.go`'s `init()` (which registers flags). The `providers` command appears with no
surgical edit to S1's scaffold. (2) This is SAFE to build in PARALLEL with S2 (P1.M4.T1.S2): S2
edits `root.go`'s `RunE` field; S3 touches a different file. Both land in package `cmd`; neither
clobbers the other. (3) It matches the surgical-footprint discipline of the milestone (S1 = scaffold,
S2 = 1-line RunE edit + 2 files, S3 = 2 new files, S4 = 2 new files for config init/path).

**Rejected: edit root.go to add the command.** That would create a merge hazard with S2 (both editing
root.go concurrently) and buys nothing — `init()` achieves the same registration.

---

## §1 — Command group shape (cobra parent + two leaf commands)

**Decision.**
```go
providersCmd     = &cobra.Command{Use:"providers", Short:"Manage AI provider manifests", ...}   // no RunE -> prints help
providersListCmd = &cobra.Command{Use:"list", Short:"List providers", Args:cobra.NoArgs, RunE:runProvidersList, ...}
providersShowCmd = &cobra.Command{Use:"show <name>", Short:"Show a provider manifest", Args:cobra.ExactArgs(1), RunE:runProvidersShow, ...}
```
`init()` does `providersCmd.AddCommand(providersListCmd, providersShowCmd); rootCmd.AddCommand(providersCmd)`.

**Why.** Matches PRD §15.3's two subcommands exactly. A parent with no `RunE` prints help on bare
`stagecoach providers` (cobra default) — conventional and consistent with the upcoming `config`
group (S4, init/path). `show` uses `cobra.ExactArgs(1)` so a missing/extra arg is a cobra arg-error
(→ exit 1, see §7) rather than a silent nil-deref. `list` uses `cobra.NoArgs` (it takes no args).

---

## §2 — Registry construction mirrors pkg/stagecoach.buildDeps EXACTLY

**Decision.** Build the registry from config the same way the public API does:
```go
overrides, err := provider.DecodeUserOverrides(cfg.Providers)   // raw config map -> typed Manifests
if err != nil { return ..., err }
reg := provider.NewRegistry(overrides)                          // built-ins ⊕ user overrides
```
Encapsulate in a `newRegistry()` helper returning `(*provider.Registry, error)`.

**Why.** This is the ONE proven bridge from `config.Config.Providers` (raw
`map[string]map[string]any`) to the typed Registry, already battle-tested by
`pkg/stagecoach.buildDeps` (pkg/stagecoach/stagecoach.go:137-142). Copying it guarantees `providers
show` displays the SAME merged manifest the generate pipeline consumes — FR47's "built-in merged
with user overrides" is the SAME object `GenerateCommit` would use. No divergence risk.

---

## §3 — Config DOES load for list/show (do NOT add to shouldSkipConfigLoad)

**Decision.** Leave `shouldSkipConfigLoad` (root.go) UNTOUCHED — it returns false for `list`/`show`,
so root's `PersistentPreRunE` runs and loads config (including user overrides) before either RunE.
`runProvidersList`/`runProvidersShow` read `Config()` (the S1 accessor for the loaded snapshot).

**Why.** FR46 requires listing "built-in + user providers"; FR47 requires "built-in merged with user
overrides". User overrides live in `cfg.Providers` (global + repo-local TOML). Skipping config would
show ONLY built-ins — directly violating both FRs. The `shouldSkipConfigLoad` exemption is reserved
for S4's `config init/path` (which manipulate the config PATH and must work outside a git repo).

**Consequence (documented gotcha).** `config.Load`'s Layer 4 (`loadGitConfig`) shells out to
`git -C <cwd> config --get` per key. Inside ANY git repo (even one with zero `stagecoach.*` keys)
that exits 1 = "missing key", which `gitConfigGet` treats as NOT-an-error → Load succeeds. OUTSIDE a
git repo it exits 128 → `loadGitConfig` returns an error → `config.Load` fails →
`PersistentPreRunE` returns `exitcode.Error` → the subcommand never runs (exit 1). Therefore
`stagecoach providers list/show` must run where `config.Load` succeeds (a git repo, or a cwd whose
git-config layer doesn't hard-fail). This is consistent with the rest of the CLI and with the
feature's need for user overrides. It is NOT a bug to fix here.

**Cobra inheritance note.** Root's `PersistentPreRunE` is INHERITED by `providers`/`list`/`show`
because none of them define their OWN `PersistentPreRunE` (cobra runs the nearest ancestor's). DO
NOT add a `PersistentPreRunE` to any of the three new commands — doing so would SHADOW root's and
skip config load (regressing §3). Verified against cobra's documented execution order.

---

## §4 — "resolved default" = cfg.Provider if set, else DefaultProvider(installed)

**Decision.** Compute the default for the list marker as:
```go
installed := installedNames(reg)                       // []string of IsInstalled==true names
var defaultName string
if cfg := Config(); cfg != nil && cfg.Provider != "" {
    defaultName = cfg.Provider                         // explicit (Layer 1-7) wins
} else {
    defaultName = reg.DefaultProvider(installed)       // auto-detect: first preferred built-in on PATH
}
```
The row whose `Name == defaultName` gets the `(default)` marker.

**Why.** "The resolved default" (PRD §15.3 / FR46) means the provider `stagecoach` would actually use
with no `--provider` flag. That resolution is EXACTLY what `pkg/stagecoach.buildDeps` does:
`name := cfg.Provider; if name == "" { name = reg.DefaultProvider(installed) }`. Showing anything
else would mislead the user. This decision uses `DefaultProvider` (the contract's named INPUT) for
the auto-detect branch AND honors an explicitly-configured `cfg.Provider` (correctness). An
explicit `cfg.Provider` pointing at a user-defined §12.8 provider is marked as default even though
`DefaultProvider` (built-ins only) would return "".

**installedNames** mirrors `buildDeps` verbatim:
```go
var installed []string
for _, m := range reg.List() { if reg.IsInstalled(m) { installed = append(installed, m.Name) } }
```

**Edge case.** If `defaultName` is set but not present in `reg.List()` (e.g. `cfg.Provider="ghost"`
but no `[provider.ghost]` anywhere — a misconfiguration), no row matches → no marker. `buildDeps`
would later error "unknown provider"; `list` simply shows the unmarked table. Acceptable; do not
special-case (the generate path is the authority on bad config).

---

## §5 — list output format: tabwriter, NAME / DETECTED(✓✗) / DEFAULT columns + header

**Decision.** Use `text/tabwriter` (stdlib) for aligned columns. Print a header row + one row per
provider (List() returns ascending-by-Name). Mark the resolved-default row with `(default)` in the
DEFAULT column:
```
NAME       DETECTED   DEFAULT
claude     ✗
codex      ✗
cursor     ✗
gemini     ✗
opencode   ✗
pi         ✓          (default)
```
(tabwriter padding 2; exact spacing is tabwriter's concern — tests assert on NAME+marker substrings,
NOT exact column alignment.)

**Why.** (1) FR46 mandates ✓/✗ for $PATH detection — use those exact glyphs (task contract: "mark
detected-on-$PATH (✓/✗)"). (2) The DEFAULT column makes the resolved default discoverable at a
glance (FR46 "show the resolved default"). (3) A header row is self-documenting and satisfies the
"Mode A — user-facing command documentation" deliverable directly in the output. (4) Sorting is
free: `Registry.List()` already sorts ascending by Name (registry.go). (5) User-defined §12.8
providers appear inline, sorted alphabetically with the built-ins — no separate section (FR48
"new names add new providers").

**stdout/stderr discipline.** The table goes to STDOUT (scriptable: `stagecoach providers list |
grep pi`). Errors/diagnostics go to STDERR via main's print (§7). Mirror S2's §5: stdout = data,
stderr = diagnostics.

**Note for P1.M4.T3.** The ✓/✗ glyphs and rows are the DATA; P1.M4.T3 may later colorize them
(green ✓ / red ✗) when TTY + !NoColor. Keep `printProvidersList(w io.Writer, …)` taking an `io.Writer`
so the UI layer can restyle/recolor without touching the resolver.

---

## §6 — show <name>: MarshalTOML → stdout, as-is (it already ends in \n)

**Decision.**
```go
s, err := reg.MarshalTOML(name)
if err != nil { return exitcode.New(exitcode.Error, fmt.Errorf("unknown provider %q", name)) }
fmt.Fprint(os.Stdout, s)   // print as-is; NO extra newline
```

**Verified output shape** (empirical, this research session, `provider.NewRegistry(nil).MarshalTOML("pi")`):
```
name = 'pi'
detect = 'pi'
command = 'pi'
subcommand = []
prompt_delivery = 'stdin'
print_flag = '-p'
model_flag = '--model'
default_model = 'glm-5-turbo'
system_prompt_flag = '--system-prompt'
provider_flag = '--provider'
default_provider = ''
bare_flags = ['--no-tools', '--no-extensions', '--no-skills', '--no-prompt-templates', '--no-context-files', '--no-session']
output = 'raw'
strip_code_fence = true
```
Properties confirmed: (a) single-quoted ASCII strings (go-toml/v2 default); (b) nil `*string`/`*bool`
pointers are OMITTED (free omitempty — e.g. pi has no `prompt_flag`? it DOES: `-p`; but it has NO
`prompt_flag` field at all — wait, `PromptFlag` is nil for pi so it's omitted; the `-p` shown is
`print_flag`). Concretely pi OMITS: `prompt_flag`, `json_field`, `retry_instruction`, `env` (all nil).
(c) empty slices marshal as `[]` (subcommand=[] for cursor/opencode→no, opencode has ["run"]; cursor
has []). (d) explicit-empty `*""` pointers marshal as `''` (`default_provider = ''` for pi). (e) the
output ENDS WITH A TRAILING `\n` (len=408, last byte='\n'). → `fmt.Fprint` (not `Fprintln`) is correct
to avoid a blank trailing line.

**Why `fmt.Fprint`, not `Fprintln`.** The trailing newline is already present; `Fprintln` would add a
second blank line. Tests assert on key substrings (`command = 'pi'`, `default_model = 'glm-5.2'`) so
they are robust to exact whitespace.

---

## §7 — Exit codes via exitcode.New/Error; never os.Exit

**Decision.** Both RunE funcs return `(error)`. Success → `return nil` (exit 0). Failures →
`return exitcode.New(exitcode.Error, err)` (exit 1). Never call `os.Exit` (only main does, via
`exitcode.For`). Unknown provider name → exit 1 (task contract: "Handle unknown provider name
(exit 1)").

**Why.** This is S1's centralized exit-code contract (`internal/exitcode/exitcode.go`): RunE returns
an error; `main` calls `os.Exit(exitcode.For(err))` and prints `stagecoach: <err>` when
`err.Error() != ""`. There is no NothingToCommit/Rescue/Timeout outcome for providers list/show —
the only exit codes are 0 (success) and 1 (config-load failure, decode error, unknown provider, cobra
arg-validation error). All of those route through `exitcode.For`'s default → 1.

**main double-print guard (same as S2 §4).** The errors we return carry a NON-empty `.Error()` (e.g.
`unknown provider "foo"`) so main prints `stagecoach: unknown provider "foo"`. That is the desired,
single, clear message — there is no separate detailed print to de-duplicate, so NO silent-ExitError
pattern is needed here (unlike S2's rescue/CAS paths). Keep it simple: return a descriptive error.

**cobra arg-validation errors.** `show` with ≠1 args → cobra returns an arg error (e.g. `accepts 1
arg(s), received 0`). `exitcode.For` on a plain cobra error → default 1. With `SilenceUsage`+
`SilenceErrors` on, cobra prints nothing; main prints `stagecoach: <cobra msg>`. Correct.

---

## §8 — Mode A docs: Short/Long help text documents output format

**Decision.** Each command gets a `Short` (one line, shown in `stagecoach --help` / `stagecoach
providers --help`) and a `Long` (multi-line, shown on the command's own `--help`). The `list` Long
explains the three columns and the default resolution; the `show` Long states TOML output + exit-1-on-
unknown. This IS the "user-facing command documentation" deliverable (Mode A).

**providers Short:** `Manage AI provider manifests`
**providers Long:** `Inspect the built-in and user-defined provider manifests Stagecoach uses to
generate commits.`

**list Short:** `List providers`
**list Long:** documents NAME / DETECTED / DEFAULT columns + how user overrides merge (FR48).

**show Short:** `Show a provider manifest`
**show Long:** `Print the fully-resolved manifest for <name> as TOML (built-in merged with user
overrides). Unknown names exit 1.`

---

## §9 — Testing: mirror root_test.go; assert on substrings + exit codes

**Decision.** `internal/cmd/providers_test.go`, `package cmd`. Drive the FULL CLI via `rootCmd` /
`Execute(context.Background())` exactly like root_test.go: `saveRootState`/`restoreRootState` +
`loadEnvSetup`/`chdir` (reuse these same-package helpers; do NOT re-copy). Capture stdout/stderr into
`bytes.Buffer`s via `rootCmd.SetOut`/`SetErr`. Derive exit code via `exitcode.For(err)`.

**Cases.**
- `TestProvidersList_Builtins`: no config → stdout contains all 6 built-in names + the header; each
  name present exactly once; sorted (claude,codex,cursor,gemini,opencode,pi).
- `TestProvidersList_InstalledDetection`: use `go` (guaranteed on PATH in `go test`) is NOT a provider
  name, so instead assert the ✓/✗ GLYPHS appear; for a deterministic installed marker, write a
  `[provider.realbin]` whose command is `go` (installed) → its row shows ✓; and a `[provider.fakebin]`
  whose command is `no-such-binary-xyz` → ✗. (Avoids depending on whether pi/claude/etc. are
  installed on the CI host.)
- `TestProvidersList_DefaultMarker`: t.Setenv STAGECOACH_PROVIDER=pi → pi row has `(default)`. Also:
  no STAGECOACH_PROVIDER + a synthetic registry where one built-in is installed → that name marked.
- `TestProvidersList_OverrideAppears`: `.stagecoach.toml` with `[provider.myagent]` → "myagent" in list.
- `TestProvidersShow_BuiltInTOML`: `providers show pi` → stdout contains `command = 'pi'` and
  `default_model = 'glm-5-turbo'`; exit 0.
- `TestProvidersShow_OverrideMerged`: `[provider.pi] default_model="glm-5.2"` → `providers show pi`
  stdout contains `default_model = 'glm-5.2'` (FR47 merge reflected).
- `TestProvidersShow_UnknownExits1`: `providers show ghost` → `exitcode.For(err)==1`.
- `TestProvidersShow_ArgCount`: `providers show` (0 args) and `providers show a b` (2 args) → exit 1
  (cobra ExactArgs).
- `TestProvidersList_NoConfigStillWorks`: inside a fresh repo with NO config files → exit 0, 6
  built-ins listed (cfg.Providers nil → DecodeUserOverrides(nil) returns empty → NewRegistry =
  built-ins only).

**State hygiene.** rootCmd is a package-level singleton — each test restores SetArgs(nil),
Out/Err writers, loadedCfg=nil, and resets Changed flags via `restoreRootState` (the existing helper).
Critical for `-race`.

**No subagent/stub needed.** Unlike S2, providers list/show do NOT invoke a generation agent — they
only touch the in-process Registry + exec.LookPath. So no `stubtest.Build` dependency. Fast, hermetic.

---

## §10 — stdout = data, stderr = diagnostics (scriptability)

**Decision.** `list` table → stdout; `show` TOML → stdout. All error messages reach the user via
main's `stagecoach: <err>` print to stderr (because the returned error is non-empty). Nothing is
printed to stdout on failure.

**Why.** A user can pipe `stagecoach providers show pi > pi.toml` or `stagecoach providers list | grep
pi`. Errors never pollute the captured stdout. Consistent with S2 §5 and PRD §15.5's pipe ethos.

---

## §11 — Do NOT touch shouldSkipConfigLoad (S4 owns it; S3 needs config)

**Decision.** Leave root.go's `shouldSkipConfigLoad` exactly as S1 wrote it (returns true only for
"init"/"path"). S3 adds NO logic there.

**Why.** (1) S3 NEEDS config loaded (§3). (2) `shouldSkipConfigLoad` is S4's surface (config
init/path). Editing it now would be scope creep into a sibling subtask and a merge hazard. (3) The
name-based skip ("init"/"path") is fragile (a "list" command named "list" is fine), but fixing that
fragility is out of scope — S3 simply doesn't use the mechanism.

---

## §12 — No conflict with S2 (parallel-safe)

**Decision.** S3 writes ONLY `internal/cmd/providers.go` + `internal/cmd/providers_test.go`. S2 edits
`internal/cmd/root.go` (RunE) + writes `internal/cmd/default_action.go{,_test.go}`. Disjoint files.

**Why this is safe in parallel.** (1) Different files → no edit conflict. (2) Both `package cmd` →
both compile into one binary; `root.go`'s `init()` (flags) + `providers.go`'s `init()` (AddCommand)
both run at startup, order-independent. (3) S2's `runDefault` is the ROOT RunE; S3's commands are
SUBCOMMANDS — cobra dispatches `stagecoach` (→ runDefault) vs `stagecoach providers list` (→
runProvidersList). No overlap. (4) Neither references the other's symbols. If S2 is NOT yet merged,
S3 still compiles and `stagecoach providers list/show` work (the root just prints help on bare
`stagecoach` until S2 lands — S1's stub). If S2 IS merged, both coexist.
