# Bug Fix Requirements

## Overview

Creative end-to-end QA was performed against the Stagecoach v2.0 PRD. The implementation was
built from source (`go build ./...` + `go test ./...` — all green) and driven end-to-end through
the real CLI binary using the bundled `cmd/stubagent` as a controllable fake agent (with a custom
shell-script stager that actually runs `git add`, enabling the full planner→stager→message→arbiter
pipeline to be exercised for real).

**What works well (validated end-to-end):**
- Single-commit happy path (snapshot → generate → commit-tree → update-ref CAS).
- Full multi-commit decomposition: auto-decompose, `--commits N`, `--single` escape, FR-M11
  single-shortcut, FR-M8 empty-skip, 1-deep overlapped staging/generation.
- Arbiter resolution paths: `null` (new commit), tip amend, mid-chain rebuild (chain correctly
  rebuilt; non-target commits keep their trees/messages), and ambiguous→null (unknown target,
  empty object, garbled output all default to a new commit).
- Safety cap (planner > `max_commits` → exit 1), planner parse failure (exit 1), duplicate
  rejection → rescue (exit 3), timeout (exit 124), CAS failure (HEAD moved → abort, no
  force-update, recovery recipe printed), `--dry-run` (exit 0/1), nothing-to-commit (exit 2).
- Binary/non-text filtering (`A\t[binary] <path>` placeholders), FR-M1 routing (staged index
  never decomposes), FR-B6 help de-duplication, config precedence (env > file).

**What does NOT work (bugs found):** four issues, one critical, that defeat core documented
behavior of the **default provider (pi)**, weaken the stager's advertised safety model, produce
misleading output after the arbiter runs, and make `config init/upgrade/path` ignore the
`--config`/`STAGECOACH_CONFIG` override. Details and exact reproductions below.

## Critical Issues (Must Fix)

### Issue 1: Provider/sub-provider conflation — `--provider <manifest-name>` is emitted and the configured `default_provider` is silently ignored (breaks the default provider, pi)

**Severity**: Critical
**PRD Reference**: §12.2 (render algorithm: `if m.provider_flag and provider != ""`), §12.3 (pi
`--provider zai|anthropic|google|...`), §9.8 FR37a (field-merge must preserve `default_provider`
across layers), §15.5 (documented `git config stagecoach.provider pi` setup), §9.16 FR-D2.
**Expected Behavior**: The `--provider` flag to a multi-provider agent (pi) must carry the
**sub-provider/backend** (`zai`, `openrouter`, …), resolved from the manifest's `default_provider`
(or an explicit sub-provider config). When a user sets `default_provider = "openrouter"`, the
rendered command must be `pi --provider openrouter …`.
**Actual Behavior**: `generate.CommitStaged` calls `deps.Manifest.Render(cfg.Model, cfg.Provider,
…)` (`internal/generate/generate.go:192`), and every decompose role does the same via
`ResolveRoleModel` (`internal/decompose/{planner,stager,message,arbiter}.go`). `cfg.Provider` is the
**manifest/agent name** (e.g. `"pi"`), not the sub-provider. Render treats any non-empty `provider`
param as the sub-provider and emits `--provider <manifest-name>`, which (a) is not a valid pi
sub-provider, and (b) **overrides and silently ignores** the user's configured `default_provider`.

This triggers in every common configuration that names pi:
- The bootstrap config written by `config init` (`[defaults] provider = "pi"` — the default
  out-of-box experience);
- `stagecoach --provider pi`;
- `git config stagecoach.provider pi` (the setup **explicitly recommended in PRD §15.5**);
- `STAGECOACH_PROVIDER=pi`.

Because the role fallback in `ResolveRoleModel` (`if provider == "" { provider = cfg.Provider }`)
also pulls in `"pi"`, **all four decompose roles** emit `--provider pi` for the bootstrap config.
The FR37a fix that preserves `default_provider` across config layers is entirely defeated: the
value survives the merge into the manifest, but Render never reads it because the caller overrides
it with the manifest name.

The shipped render unit test (`TestRender_GoldenPerProvider` / `TestRender_PersonalOverride`) passes
only because it invokes `Render` *directly* with the sub-provider string (`""` or `"zai"`), bypassing
the caller conflation — so CI does not catch it.

**Steps to Reproduce** (uses the stub as pi so no real model call is made):
```bash
mkdir -p /tmp/repro && cat > /tmp/repro/config.toml <<'TOML'
config_version = 2
[defaults]
provider = "pi"
[provider.pi]
command = "<repo>/bin/stubagent"
detect  = "<repo>/bin/stubagent"
provider_flag = "--provider"
default_provider = "openrouter"     # <<< the value that MUST be honored
model_flag = "--model"
default_model = "gpt-5.4-nano"
system_prompt_flag = "--system"
prompt_delivery = "stdin"
print_flag = "-p"
output = "raw"
[provider.pi.env]
STAGECOACH_STUB_OUT = "feat: repro"
TOML
cd <fresh git repo with one staged change>
STAGECOACH_CONFIG=/tmp/repro/config.toml stagecoach --dry-run --verbose --no-color
# Observed:  DEBUG: command: ...stubagent --provider pi --model gpt-5.4-nano --system-prompt …
# Expected:  ...stubagent --provider openrouter --model gpt-5.4-nano …
```
Reproduced three independent ways (bootstrap-style config, `default_provider="openrouter"`, and
`git config stagecoach.provider pi` + `[provider.pi] default_provider="zai"`); all emitted
`--provider pi` and ignored the configured sub-provider.

**Suggested Fix**: Separate the two concepts at the call boundary. `cfg.Provider` is the registry
key (manifest name); the sub-provider for rendering is the manifest's resolved `DefaultProvider`
(unless a distinct sub-provider field/config layer supplies one). Pass `""` (or a dedicated
sub-provider value) to `Render`'s `provider` parameter so Render falls back to the merged
`DefaultProvider`. Concretely, in `generate.go:192` and the four decompose role files, stop passing
`cfg.Provider`/`ResolveRoleModel`'s provider as the Render sub-provider; instead let Render resolve
it from the manifest (or thread a real sub-provider through config). Add an integration test that
drives the CLI with a pi-shaped stub and asserts `--provider <default_provider>` (not the manifest
name) is emitted.

## Major Issues (Should Fix)

### Issue 2: The stager's toolset is not actually scoped — PRD §19's "structurally constrained, cannot commit/amend/push" claim does not hold

**Severity**: Major (safety / security claim gap)
**PRD Reference**: §11.5 ("a git/read/edit allowlist expressed via `tooled_flags`"), §17.6
("structurally constrained"), §19 ("Its toolset is scoped … it cannot commit, amend, or push,
because stagecoach owns every ref mutation"), §22.1 risk row ("Scoped git toolset
(`tooled_flags`)").
**Expected Behavior**: The tooled stager agent can run only staging-relevant operations
(`git add`, read, edit) and is structurally unable to commit, amend, push, or move refs.
**Actual Behavior**: The shipped `tooled_flags` do not scope the stager:
- **pi** (`internal/provider/builtin.go`, `builtinPi`): `tooled_flags` = bare flags **minus**
  `--no-tools` — i.e. pi's **entire native tool system is enabled with no allowlist**. The inline
  comment admits this: *"pi has no git-scoped allowlist … stager safety is via the stager task
  prompt + stagecoach's ref-mutation monopoly, not flag-scoping."* A pi stager can run arbitrary
  Bash, including `git commit`, `git push`, `git update-ref`, `rm -rf`.
- **claude** (`builtinClaude`): `tooled_flags = ["--allowed-tools", "Bash(git:*),Read,Edit", …]`.
  `Bash(git:*)` permits **every** git subcommand — `git commit`, `git push --force`,
  `git update-ref HEAD`, `git reset --hard HEAD~5` — so it does NOT prevent commit/amend/push.

"stagecoach's ref-mutation monopoly" is only true if the agent cannot mutate refs itself; with these
profiles a misbehaving stager can. The structural guarantee the PRD sells (§19) is therefore not
delivered for either stager-capable provider.
**Steps to Reproduce / Inspect**: `stagecoach providers show pi` (or read `providers/pi.toml`) →
`tooled_flags` has no tool allowlist; `stagecoach providers show claude` → `Bash(git:*)`. Compare to
§19/§11.5 prose.
**Suggested Fix**: Either (a) tighten the profiles so commit/push/ref-mutation are genuinely
unreachable (e.g. claude `Bash(git add:*,git apply:*,git status:*,git diff:*)` instead of
`Bash(git:*)`; for pi, document the unsoped risk honestly and downgrade the §19 claim to
"instructional + best-effort"), or (b) add a defensive guard (e.g. snapshot HEAD before each stager
call and abort if HEAD moved after the stager returns). At minimum, correct the PRD/§19 language so
the structural claim matches reality.

### Issue 3: After the arbiter runs, stdout prints stale (now-dangling) commit SHAs and omits the arbiter's new commit entirely

**Severity**: Major (correctness of user-facing output)
**PRD Reference**: §13.6.5 / FR-M9–M10 (arbiter reconciliation), §15.4 / FR42 (success report),
Appendix B.1 (the `[<short-sha>] <subject>` + file-list report).
**Expected Behavior**: The user-facing output reflects the **final** state of the repository after
the arbiter amends/rebuilds/creates. At minimum, the printed SHAs must exist, and a commit the
arbiter creates should be reported.
**Actual Behavior**: `runDecompose` prints `res.Commits` *as returned by the loop*, before/without
reflecting the arbiter's changes:
- **Tip / mid-chain amend**: the amended commit gets a **new SHA**, but stdout still prints the
  **old (pre-amend) SHA**, which is now dangling. Example: stdout `[ac13570] feat: msg for concept 2`
  while `git log` shows the real tip as `c8eddbf`. The user is shown a SHA that no longer resolves.
- **Null / new-commit path**: the arbiter creates an (N+1)-th commit (e.g. for leftover `d.txt`),
  which lands in `git log` but is **never printed** to the user at all.

This is acknowledged internally (decompose.go `§G-RESULT`) as a "post-arbiter gap," but from the
user's perspective the success report is incorrect/incomplete after any arbiter action — the most
common decompose outcomes (leftovers reconciled).
**Steps to Reproduce** (stub planner partitions b.txt+c.txt; arbiter folds leftover d.txt into the
tip; smart arbiter returns the tip SHA):
```bash
# planner: 2 concepts (b,c); arbiter mode=tip (returns HEAD sha); d.txt is leftover
stagecoach   # nothing staged, dirty tree (b.txt, c.txt, d.txt)
# stdout:  [<sha-A>] … b.txt   [<sha-B>] … c.txt      (sha-B is STALE after amend)
# git log: …<sha-B'> (amended tip, c.txt + d.txt), <sha-A> …      (sha-B' != sha-B)
```
Reproduced for tip, mid-chain (folded into concept 1; concept 2 rebuilt — logic correct, but
stdout SHAs stale), and null (3rd commit created but not printed).
**Suggested Fix**: After a successful arbiter phase, re-read git for the final commits (e.g.
`git log <preRunHEAD>..HEAD`) and print those SHAs/subjects/file-lists, replacing the loop's
pre-amend entries. This is straightforward given stagecoach already owns every ref mutation.

### Issue 4: `config init` / `config upgrade` / `config path` ignore `--config` and `STAGECOACH_CONFIG` and always operate on the global config path

**Severity**: Major
**PRD Reference**: §15.2 (`--config` / `STAGECOACH_CONFIG` "Path to a config file, overrides
discovery"), §9.8 FR38 (`config init`/`config path`/`config upgrade`).
**Expected Behavior**: `stagecoach --config X config upgrade` (or `STAGECOACH_CONFIG=X`) upgrades
file `X`; `config path` reports the resolved (overridden) path.
**Actual Behavior**: The config subcommands compute their target from `GlobalConfigPath()`
(`$XDG_CONFIG_HOME` or `$HOME/.config/stagecoach/config.toml`) and never consult `flagConfig` /
`STAGECOACH_CONFIG`:
```
$ stagecoach --config /tmp/cfg_alt/c.toml config upgrade
Config at /home/…/.config/stagecoach/config.toml is already at version 2 (no changes).
# /tmp/cfg_alt/c.toml is NOT touched.
$ STAGECOACH_CONFIG=/tmp/cfg_alt/c.toml stagecoach config path
/home/…/.config/stagecoach/config.toml          # lies — reports global, not the override
```
Consequence: a user who drives stagecoach with a custom/repo config and then runs
`config upgrade` silently mutates (or creates) their **global** config instead of the intended file,
and `config path` misleads debugging. (`config init`/`upgrade` *do* honor `HOME`/`XDG`; only the
`--config`/`STAGECOACH_CONFIG` override is dropped.)
**Steps to Reproduce**: set `STAGECOACH_CONFIG` (or pass `--config`) to a non-default file, then run
`config path` / `config upgrade` and observe they reference the global path.
**Suggested Fix**: Have the config subcommands resolve their target via the same
override-aware path resolver used by the default action (honor `flagConfig` → `STAGECOACH_CONFIG` →
`GlobalConfigPath()`), so `config path`/`init`/`upgrade` operate on the file the user actually
selected.

## Minor Issues (Nice to Fix)

### Issue 5: The bootstrap config for the default provider (pi) is not functional out-of-the-box for generation (latent; surfaces once Issue 1 is fixed)

**Severity**: Minor
**PRD Reference**: §9.17 FR-B1 ("writes a populated, **working** config … the tool works
immediately"), §9.16 FR-D4 / Appendix E #12.
**Expected Behavior**: After `config init` (which auto-detects pi and writes `gpt-5.4*` per-role
models), `stagecoach` produces a valid pi invocation.
**Actual Behavior**: `config init` writes `[defaults] provider = "pi"` and `[role.*] model =
"gpt-5.4"/"gpt-5.4-mini"/"gpt-5.4-nano"` but **no** `default_provider` (sub-provider). Combined with
Issue 1 (currently masked because `--provider pi` errors first), once rendering stops emitting the
bogus sub-provider, pi will receive `pi --model gpt-5.4-nano …` with **no** `--provider`, so pi
routes `gpt-5.4-nano` to its own default backend (per the pi.toml comment, "google"), where that
model does not exist → model-not-found. FR-B1's "works immediately" promise is not met for the
default provider's multi-commit (and single-commit) path. (This is partly the documented Appendix E
#12 open question, but the shipped bootstrap should not write models that cannot route.)
**Suggested Fix**: Either write a `default_provider` (the OpenAI-routing pi sub-provider, once
verified per Appendix E #12) into the bootstrap `[provider.pi]`, or default pi's per-role models to
`""` (so pi picks its own backend default) until a verified routing sub-provider exists.

## Testing Summary

- **Total tests performed**: ~30 distinct end-to-end scenarios across the single-commit and
  multi-commit pipelines, config/provider CLI, exit-code matrix, and security-model inspection.
- **Passing**: ~26 (all core happy-path, edge-case, error-handling, and atomicity behaviors listed
  in the Overview).
- **Failing**: 4 documented above (1 Critical, 3 Major) + 1 Minor.
- **Areas with good coverage**: snapshot/atomic-commit core, CAS/timeout/rescue, duplicate
  rejection, binary filtering, the full planner→stager→message→arbiter pipeline including all three
  arbiter resolution paths and ambiguous-defaulting, exit codes, FR-M1 routing, config precedence,
  FR-B6 help de-dup.
- **Areas needing more attention**: the provider-render call boundary (Issue 1 — the highest-impact
  gap; unit tests bypass it so it escaped detection), the stager toolset scoping vs. the §19 security
  claim (Issue 2), post-arbiter output fidelity (Issue 3), and `--config` honoring in config
  subcommands (Issue 4).

**Note on the test method**: Issues 1, 3, and 4 were reproduced through the real CLI binary using
`cmd/stubagent` as a stand-in agent (with `--verbose` to inspect the rendered command), so the
reproductions require no real model calls. Issue 2 is a static inspection of the shipped manifests
(`internal/provider/builtin.go` / `providers/*.toml`) against the PRD's stated safety properties.
