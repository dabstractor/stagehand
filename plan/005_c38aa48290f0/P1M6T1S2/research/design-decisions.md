# P1.M6.T1.S2 — `stagecoach models [<provider>]`: Design Decisions & Evidence

> Companion to `../PRP.md`. Captures the research that drove each decision so the PRP's tasks read as
> obvious, and so a re-plan after a failure has the reasoning to pivot on.

## 1. The S1 contract (the field this command consumes)

P1.M6.T1.S1 adds `Manifest.ListModelsCommand []string` (`toml:"list_models_command"`) to
`internal/provider/manifest.go`, merged by slice-regime-2 (non-empty override REPLACES wholesale),
populated on 4 verified built-ins (`opencode=["opencode","models"]`, `pi=["pi","--list-models"]`,
`agy=["agy","models"]`, `cursor=["agent","models"]`), absent on the other 4. **Treat S1 as delivered.**
This command reads `m.ListModelsCommand` from the merged `Registry.Get(name)` manifest. No new field is
added by S2; S2 only adds the consumer + the verification-date constant (§6).

S2 REUSES (no duplication):
- `newRegistry()` / `installedNames(reg)` / `resolvedDefault(cfg, reg, installed)` — already in
  `internal/cmd/providers.go` (same package `cmd`). Import-free reuse.
- `Registry.Get/IsInstalled/DefaultProvider` — `internal/provider/registry.go`.
- `config.DefaultModelsForProvider(name)` — the curated FR-D4 fallback (`internal/config/role_defaults.go`).

## 2. Command structure → mirror `providers.go` (the established pattern)

`internal/cmd/providers.go` is the exact template: a `&cobra.Command{Use, Short, Long, Args, RunE}`,
registered on `rootCmd` inside `init()` (NO edit to root.go — the providers comment says "register on
S1's root — NO edit to root.go"). S2's `modelsCmd` is a single LEAF on root (not a group), so even
simpler. `PersistentPreRunE` loads config for every command except `init/path/upgrade` + help/version
(`shouldSkipConfigLoad` in root.go) — `models` is NOT in that set, so **config loads** (needed for
`cfg.Providers` overrides + `cfg.Provider` default + `cfg.Timeout`). Confirmed by reading root.go.

Exit-code convention (providers.go): `return exitcode.New(exitcode.Error, fmt.Errorf(...))` → main maps
to exit 1. Never `os.Exit`. S2 follows this for all error paths (§5).

## 3. The `--all` flag collision — RESOLVED via the `config init` precedent

**The problem:** root.go defines `--all`/`-a` as a PERSISTENT flag (`flagAll`, "git add -A"). FR-L1
requires `models --all` ("every detected provider"). A naive local `--all` would seem to collide.

**The resolution (proven in THIS codebase):** `config init` (`internal/cmd/config.go:142`) defines a
LOCAL `--provider` flag that overrides root's persistent `--provider`, and root.go's own comment
documents the mechanism: *"pflag's AddFlagSet skips this inherited persistent flag on `config init`
since a local name already exists."* So pflag/cobra's `AddFlagSet` SKIPS a parent persistent flag when
the child already defines a flag of the same name — **no panic, child's help text wins.**

Therefore: `modelsCmd.Flags().BoolVarP(&flagModelsAll, "all", "a", false, "<models semantics>")`
correctly overrides the inherited persistent `--all`/`-a` for the `models` command ONLY, with the
correct help text. `models` never runs the default action, so `flagAll` being skipped is irrelevant.
**This is the established, tested pattern — not a new gamble.**

## 4. Running the list command → reuse `provider.Execute` (the executor seam)

FR-L1 order: (a) run `list_models_command`, print stdout; (b) on absent/failure, fall back to the
curated table. The contract lists "the executor seam (or a plain exec)". 

**Decision: use `provider.Execute`** (`internal/provider/executor.go`, already imported by
`internal/cmd/providers.go`). Build `provider.CmdSpec{Command: argv[0], Args: argv[1:], Stdin: ""}`
with `Env` nil (nil ⇒ child inherits parent env — exactly "inherited env"). Pass `cfg.Timeout` as the
bound (Execute wraps `context.WithTimeout` internally; on expiry returns `context.DeadlineExceeded`,
which S2 treats as "command failed" → curated fallback). Rationale:
- DRY + already-tested process-group kill (handles grandchildren, `setupProcessGroup`), timeout, and
  stdout/stderr capture — `executor_test.go` proves it with stub binaries (`cat`, `sleep`, `false`).
- `signal.RegisterChild/ClearChild` (called inside Execute) are harmless for a non-generate-flow
  subcommand: the rescue signal handler is armed only in the generate pipeline, not for `models`, so
  the package-global child slot is set then cleared with no observer.
- Optional `--verbose`: build `ui.NewVerbose(stderr, cfg.Verbose)` and pass it so `stagecoach -v models`
  prints `DEBUG: command:` + `DEBUG: raw output:` — consistent with the rest of stagecoach. Pass nil if
  cfg is nil/verbose off.

Alternative (documented in PRP, not chosen): a local `exec.CommandContext` helper. Rejected as primary
because it would either (a) skip process-group kill (grandchildren may leak) or (b) duplicate the
platform-specific `setupProcessGroup`/`procgroup_*.go` machinery. `provider.Execute` already owns that.

## 5. Error / exit-code matrix (from the work-item contract: "unknown or undetected provider → clear error, nonzero exit")

| Invocation | Condition | Result |
|---|---|---|
| `models <p>` | `p` not in registry | error `unknown provider "p"` → exit 1 |
| `models <p>` | `p` known but NOT on PATH | error `provider "p" is not detected on $PATH …` → exit 1 |
| `models <p>` | known + detected | one block (live list → else curated table) → exit 0 |
| `models` | default resolves (something detected) | one block for the default → exit 0 |
| `models` | nothing detected (default == "") | error `no provider detected on $PATH …` → exit 1 |
| `models --all` | ≥1 detected | one block per detected provider → exit 0 |
| `models --all` | nothing detected | error `no providers detected on $PATH` → exit 1 |
| `models --all <p>` | both given | usage error `--all cannot be combined with a provider argument` → exit 1 |
| live command times out / non-zero / not-found | (per provider) | **stderr** notice "list command failed; using curated table"; **stdout** gets the curated table → exit 0 (fallback succeeded) |
| detected provider, no `list_models_command`, no curated table (user-defined) | info `provider "x" has no list_models_command and no curated defaults …` → exit 0 (successful but empty) |

All errors go through `exitcode.New(exitcode.Error, err)` (exit 1), never `os.Exit` (matches providers.go).

## 6. The curated-table fallback + verification-date annotation

`config.DefaultModelsForProvider(name)` returns `map[string]string{planner,stager,message,arbiter→model}`
for any built-in name (nil for user-defined). Render in a FIXED role order `[planner, stager, message,
arbiter]` (NOT map iteration — map order is random). Empty stager cell (gemini/agy/opencode/etc. — not
stager-capable) renders as `—`.

**Annotation requirement (FR-L1):** "annotated with its verification date and 'consult `<command>
--help` for the live list'." The verification date today lives ONLY as a comment in role_defaults.go
(`Verification date: 2026-07-02`). To surface it as DATA, S2 adds ONE exported constant
`config.DefaultModelsVerificationDate = "2026-07-02"` (matching the comment; FR-D5 refresh discipline —
update constant + table together). `<command>` = the provider's `m.DetectCommand()` (e.g. `claude`,
`opencode`), so the hint reads `consult \`claude --help\``. Consistent whether the command was absent or
failed.

## 7. Test strategy (contract: "stub a fake listing binary on PATH; fallback rendering golden test")

Mirrors `internal/cmd/providers_test.go` helpers: `setupRepo`, `saveRootState`/`restoreRootState`,
`writeConfigFile`, `Execute(context.Background())` with `rootCmd.SetArgs`. All deterministic (no network
— N2).

1. **Live-list happy path (stub binary):** put a fake `opencode` (shell script printing
   `STAGECOACH_FAKE_MODELS`) on a temp PATH; `stagecoach models opencode` (opencode is detected because
   the fake is on PATH) → assert `STAGECOACH_FAKE_MODELS` appears under the `opencode:` heading.
   Alternatively/also: a user-defined `[provider.stublist]` with `list_models_command=["stubbin","models"]`.
2. **Fallback golden test (no binary needed):** `stagecoach models claude` (claude has no
   `list_models_command`, NOT detected → would error). To test the FALLBACK specifically, use a
   user-defined `[provider.fakep] command="stubbin"` with NO `list_models_command` and a fake `stubbin`
   on PATH (so it's detected) → assert the curated-style block prints. **Cleaner: test the pure
   renderer directly** (`printModelBlock(w, name, manifest, liveStdout, fallbackDate)`) with a fixed
   manifest + `DefaultModelsForProvider("claude")` → golden-string assertion (no PATH juggling). Do
   BOTH: a renderer unit test (deterministic golden) + a CLI integration test for the fallback path.
3. **Command-FAILURE fallback:** fake `opencode` that `exit 1` → `stagecoach models opencode` → curated
   table on stdout + a stderr notice. Assert stdout has the curated footer AND stderr has the notice.
4. **Error cases:** unknown provider (exit 1), undetected named provider (exit 1), bare `models` with
   nothing detected (exit 1), `--all` with nothing detected (exit 1), `--all` + arg (usage error exit 1).
5. **`--all` multi-provider:** ≥2 detected → both headings appear, blank-line separated.

## 8. Why no external/online research was needed

The PRD (§9.23 FR-L1/L2, §9.16 FR-D4, §6.2 N2, §15.3) fully specifies behavior. The implementation is a
cobra command following `internal/cmd/providers.go` (the exact template, same package), consuming S1's
`ListModelsCommand` field + the existing `DefaultModelsForProvider`. The only non-obvious point (the
`--all` persistent/local collision) is resolved by an in-codebase precedent (`config init --provider`).
There is no new library, no new API surface, no external integration to research. Spawning online
subagents would add latency without information.
