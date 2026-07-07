# Bug Fix Requirements

## Overview

Second-pass end-to-end QA / bug-hunting run against the Stagecoach v1.0 implementation vs.
`PRD.md`, performed *after* the bugfix-001 changeset landed. All seven bugfix-001 issues were
re-verified as **fixed and non-regressed**: `--config` is honored by the default action (001 #1),
`--dry-run` now runs the full dedupe/retry loop and takes the snapshot (001 #2/#6), a missing
provider command fails fast with exit 1 before the snapshot (001 #3), `[generation]`
output/strip_code_fence are bridged onto the manifest (001 #4), the repo-local redirect notice
prints once (001 #5), and the clean-tree "(0 files)" notice is suppressed (001 #7). The
data-integrity core remains rock solid: snapshot-based atomic commit, stage-while-generating,
CAS failure (HEAD unchanged), rescue (exit 3), timeout (exit 124), SIGINT→rescue, merge-conflict
abort, and root-commit (unborn repo) all behave correctly. The entire `go test -race ./...` suite
passes and `go vet` is clean.

The two issues below are **not** data-integrity or crash bugs. They are **silent wrong-behavior
bugs** found by driving the real binary through documented user journeys. Both center on the
config↔manifest handoff seam:

- **Bug A** — an *explicit* `--config`/`STAGECOACH_CONFIG` path pointing at a **nonexistent** file
  is silently swallowed as "layer absent" (the same code path used for *discovery*), so Stagecoach
  falls back to auto-detection and **silently invokes a real, installed AI agent** instead of
  erroring.
- **Bug B** — the bugfix-001 "Issue 4" bridge copies `cfg.Output`/`cfg.StripCodeFence` onto the
  resolved manifest *unconditionally*, but both fields carry non-empty/non-nil **defaults**, so the
  `[generation]` default always wins. A **manifest-level** `output = "json"` or
  `strip_code_fence = false` is silently clobbered — JSON output mode (PRD §12.4/§12.9) is dead at
  the manifest level, and `stagecoach providers show` *claims* `output = 'json'` while parsing
  actually uses raw.

- **Build:** `go build ./...` clean; `go vet ./...` clean.
- **Tests:** `go test -race ./...` — all packages pass (incl. `TestSignalIntegration_SigintPostSnapshot`).
- **Binaries used:** `bin/stagecoach` (rev `dev`) + `bin/stubagent`; real git repos in tmpdirs.

## Critical Issues (Must Fix)

None. No data corruption, no crashes, no broken data-integrity invariants. The snapshot/atomic-
commit core (§13/§18.1), rescue/timeout/CAS error mapping (§15.4/§18.2/§18.3), signal handling
(§18.4), and root-commit path all work correctly.

## Major Issues (Should Fix)

### Issue 1: An explicit `--config` / `STAGECOACH_CONFIG` pointing at a NONEXISTENT file is silently ignored → Stagecoach invokes a real agent instead of erroring

**Severity**: Major
**PRD Reference**: §15.2 (`--config <path>` — "Path to a config file (overrides discovery)");
§16.1 layer resolution (the override layer); §13.5 ("on direct use, fail fast … exit non-zero");
§19 (no surprise provider redirection). The intent of "overrides discovery" is that the named file
*is* the config.
**Expected Behavior**: When the user *explicitly* passes `--config <path>` (or sets
`STAGECOACH_CONFIG=<path>`) and that file does not exist, Stagecoach should fail fast with a clear
error (exit 1) naming the missing path — exactly as it already does for a *malformed* file or a
*directory*. A typo like `--config confgi.toml` must not silently change which agent runs.
**Actual Behavior**: A missing explicit config is indistinguishable from "the *discovery* default
file isn't there yet." `config.Load` (internal/config/load.go:40-52) resolves `globalPath` from the
override, then calls `loadTOML(globalPath)`; `loadTOML` returns `(nil, nil)` on `os.IsNotExist`
(internal/config/file.go:100) and `Load` treats `nil` as "layer absent, no error." With no provider
resolved, `pkg/stagecoach.buildDeps` auto-detects the first **installed built-in** (pi/claude/gemini/
… — all present on this machine) and **invokes the real agent**. The user's wait, API call, and
quota are consumed by an agent they never asked for, and (without `--dry-run`) it may even *commit*
with a real generated message.
**Steps to Reproduce**:
```bash
cd /tmp && rm -rf cfgbug && mkdir cfgbug && cd cfgbug
git init -q && git config user.email t@t.com && git config user.name t
git commit -q --allow-empty -m init && echo "content here" > file.txt && git add file.txt
SH=/home/dustin/projects/stagecoach/bin/stagecoach

# (a) nonexistent --config  -> exit 0, a REAL agent is invoked (a genuine LLM message appears):
$SH --config /tmp/cfgbug/DOES_NOT_EXIST.toml --dry-run
# ↳ Generating…
# feat(file): add initial content file        ← produced by a real installed agent, NOT a stub
# (no commit created)
# exit 0                                       ← should be exit 1 "config file not found"

# (b) STAGECOACH_CONFIG to a nonexistent file behaves identically (exit 0, real agent):
STAGECOACH_CONFIG=/tmp/cfgbug/NOPE2.toml $SH --dry-run    # exit 0

# Contrast — these correctly ERROR:
$SH --config /tmp/cfgbug/somedir    --dry-run   # directory -> exit 1 "is a directory"
printf 'bad [toml' > bad.toml && $SH --config bad.toml --dry-run   # malformed -> exit 1 "expected '='"
```
So a *malformed* or *wrong-type* explicit path errors, but a *missing* one is silently ignored — an
inconsistent and dangerous special case.
**Suggested Fix**: In `config.Load` (internal/config/load.go), distinguish the *explicit override*
path from the *discovery* path. When `opts.ConfigPathOverride != ""` or `STAGECOACH_CONFIG` was the
source of `globalPath`, a missing file must be a hard error, e.g.:
```go
explicit := opts.ConfigPathOverride != "" || os.Getenv("STAGECOACH_CONFIG") != ""
g, err := loadTOML(globalPath)
if err != nil { return nil, fmt.Errorf("global config: %w", err) }
if g == nil && explicit {
    return nil, fmt.Errorf("config file not found: %s", globalPath)
}
```
Only the *discovery* path (layer-2 default `globalConfigPath()`) should tolerate absence. (The
existing `loadTOML` `IsNotExist → (nil,nil)` contract is preserved for discovery.)

### Issue 2: Manifest-level `output` and `strip_code_fence` are silently overridden by the `[generation]` defaults (JSON output mode is dead at the manifest level)

**Severity**: Major
**PRD Reference**: §12.1 / §12.9 (`parseOutput` reads the **manifest's** `output` /
`strip_code_fence`); §12.4 (Claude Code `output = "json"` + `json_field = "result"` is a documented
per-provider option); §17.4 (JSON mode remains available for agents where `--output-format json` is
more reliable); Appendix D quick-reference table (`output` is a per-provider column). The bugfix-001
"Issue 4" bridge intended to make these *overridable* via `[generation]`; instead it makes
`[generation]` *always win*, even when the user never set it.
**Expected Behavior**: `output` / `strip_code_fence` are per-manifest settings. A user who sets
`output = "json"` (+ `json_field`) on a `[provider.X]` block gets JSON parsing for that provider
without also having to repeat `output = "json"` under `[generation]`. Symmetrically,
`strip_code_fence = false` on a provider is honored.
**Actual Behavior**: `pkg/stagecoach.buildDeps` (pkg/stagecoach/stagecoach.go:206-211) copies
`cfg.Output`/`cfg.StripCodeFence` onto the resolved manifest *unconditionally*:
```go
if cfg.Output != "" {            // cfg.Output is ALWAYS "raw" — Defaults() sets it (config.go:68)
    o := cfg.Output; m.Output = &o
}
if cfg.StripCodeFence != nil {   // ALWAYS non-nil — Defaults() sets boolPtr(true) (config.go:69)
    m.StripCodeFence = cfg.StripCodeFence
}
```
Both guards always pass because the layer-1 defaults are non-empty / non-nil, so the manifest's own
`output`/`strip_code_fence` are **always overwritten**. Consequences, verified end-to-end:
- A manifest with `output = "json"` + `json_field` and **no** `[generation]` block → JSON is NOT
  parsed; the raw JSON blob is returned verbatim as the commit message. JSON mode only "works" if the
  user *also* sets `[generation] output = "json"` (a global override) — i.e. manifest-level JSON is
  dead, and you cannot mix raw/json providers.
- A manifest with `strip_code_fence = false` and no `[generation]` block → the fence is stripped
  anyway (the default `true` wins).
- `stagecoach providers show <name>` prints `output = 'json'` (it shows the *registry's* merged
  manifest, pre-override), so the displayed config **lies** about what parsing will do — a serious
  debugging trap.

(The bugfix-001 commit `dc46924` "promote strip-code-fence to tri-state pointer to honor explicit
false" attempted to fix the fence half, but `Defaults()` still initializes `StripCodeFence` to a
non-nil `&true`, defeating the tri-state nil-check in `buildDeps`. `Output` was never made
tri-state at all.)
**Steps to Reproduce**:
```bash
cd /tmp && rm -rf b3 && mkdir b3 && cd b3
git init -q && git config user.email t@t.com && git config user.name t
git commit -q --allow-empty -m init && echo a > a.txt && git add a.txt
SH=/home/dustin/projects/stagecoach/bin/stagecoach
STUB=/home/dustin/projects/stagecoach/bin/stubagent
cat > config.toml <<EOF
[defaults]
provider = "stub"
timeout = "10s"
[provider.stub]
command = "$STUB"
prompt_delivery = "stdin"
output = "json"
json_field = "msg"
[provider.stub.env]
STAGECOACH_STUB_OUT = '{"msg": "feat: json claim"}'
EOF
# providers show CLAIMS json:
$SH --config config.toml providers show stub | grep ^output      # -> output = 'json'
# but parsing returns RAW (unparsed JSON):
$SH --config config.toml --dry-run 2>/dev/null                    # -> {"msg": "feat: json claim"}
# Adding [generation] output = "json" makes it work (proving the manifest setting was clobbered):
printf '[generation]\noutput = "json"\n' | cat - config.toml > cfg2.toml
$SH --config cfg2.toml --dry-run 2>/dev/null                      # -> feat: json claim   ✓
```
**Suggested Fix**: Make the `[generation]` override genuinely optional (tri-state), so "user did not
set it" leaves the manifest's value intact. Concretely: (a) make `Config.Output` a `*string`
(nil ⇒ honor manifest; non-nil ⇒ override) and stop defaulting it in `Defaults()` (let the manifest's
`output` be the source of truth, with `raw` as the manifest-level default per §12.1); and (b) for
`StripCodeFence`, do **not** initialize it to `boolPtr(true)` in `Defaults()` — leave it `nil` so the
`buildDeps` nil-check ("only override when the user explicitly set `[generation]` strip_code_fence")
actually means "use the manifest's resolved value." Both changes ensure the manifest setting is the
default and `[generation]` is a true opt-in override, matching §12.1/§12.9 and making
`providers show` truthful. (Whichever direction is chosen, the current state — `providers show`
reports one value while parsing uses another — must be eliminated.)

## Minor Issues (Nice to Fix)

### Issue 3: Unresolved merge conflicts surface a raw, multi-line `git write-tree` error instead of the PRD's clean "resolve merge conflicts first" message

**Severity**: Minor
**PRD Reference**: §13.5 ("Unresolved merge conflicts in the index: `write-tree` fails. Stagecoach
aborts before any generation with 'resolve merge conflicts first.'") and the §18.2 failure table
(merge conflicts → exit 1).
**Expected Behavior**: On unresolved conflicts, print a single clean line — "Resolve merge
conflicts first." — and exit 1, before the model is invoked.
**Actual Behavior**: The behavior is *correct* (exit 1, the model is never invoked, HEAD/index are
untouched), but the user sees the raw, noisy git plumbing error (multiple `f.txt: unmerged (...)`
lines + `fatal: git-write-tree: error building trees`) instead of the friendly message, and the
"↳ Generating with <provider>…" progress label prints *before* the failure (cosmetic — `write-tree`
is step 3, still pre-generation).
**Steps to Reproduce**: create a content conflict, leave it unresolved, run `stagecoach`.
```bash
git init -q && git config user.email t@t.com && git config user.name t
printf 'l\n' > f.txt && git add f.txt && git commit -q -m init
git checkout -q -b o && printf 'lo\n' > f.txt && git commit -q -am o
git checkout -q master && printf 'lm\n' > f.txt && git commit -q -am m
git merge o            # -> CONFLICT
# configure any stub provider, then:
stagecoach --config config.toml
# ↳ Generating with stub…
# stagecoach: git write-tree: unresolved merge conflicts in index (exit 128): f.txt: unmerged (…)
# f.txt: unmerged (…)
# f.txt: unmerged (…)
# fatal: git-write-tree: error building trees        (exit 1)
```
**Suggested Fix**: Detect the "unmerged entries" condition (e.g., `git ls-files -u` non-empty, or
recognize `write-tree`'s exit-128 "unmerged" error) *before* calling `WriteTree`, and return the
single-line §13.5 message. At minimum, wrap the `write-tree` error to the friendly text when the
underlying cause is unmerged paths.

### Issue 4: `--dry-run` can exit 3 (rescue) or 124 (timeout) — surprising for a "preview" command

**Severity**: Minor
**PRD Reference**: §9.12 / FR49 (dry-run runs the "full … pipeline … print the resulting message,
do not create the commit"); §15.4 exit codes.
**Expected Behavior**: A user running `stagecoach --dry-run` to preview a message reasonably expects
either a message (exit 0) or a clear "couldn't generate" outcome. Because dry-run now (correctly,
per the bugfix-001 fix) runs the full pipeline including the snapshot, a parse-failure/duplicate-
exhaustion or timeout surfaces as the **full §18.3 rescue block** (Tree ID + manual `commit-tree`
recovery command) and exit 3 / 124.
**Actual Behavior**: Verified: `--dry-run` against a stub that sleeps past the timeout exits **124**
and prints the rescue recipe; against a stub that always returns empty/unparseable output it exits
**3** with the rescue recipe. Functionally defensible (the snapshot was taken; FR49 says run the full
pipeline), but the "manual recovery" recipe is odd for an operation that was never going to commit,
and a script doing `msg=$(stagecoach --dry-run)` will get a non-zero exit + no message on stdout.
**Steps to Reproduce**:
```bash
# stub provider, STAGECOACH_STUB_SLEEP_MS=5000, config timeout = "1s":
stagecoach --dry-run          # -> exit 124 + full rescue block (Tree ID, git commit-tree | xargs ...)
# stub provider, STAGECOACH_STUB_OUT = "" (always unparseable):
stagecoach --dry-run          # -> exit 3 + rescue block
```
**Suggested Fix**: Consider a dry-run-specific outcome on generation failure (e.g., exit 1 with a
short "could not generate a message; run without --dry-run to see retries/rescue", and omit the
manual commit-tree recovery recipe since no commit was ever intended). If the current behavior is
intentional, document it explicitly under `--dry-run`.

## Testing Summary

- **Total distinct scenarios exercised**: ~30 (happy-path commit on mature + unborn/root repo;
  dry-run message-on-stdout/no-commit/stderr-notice; `--no-auto-stage` → 2; clean-tree auto-stage
  → 2 with no "(0 files)" notice; missing-provider-command fail-fast → 1; timeout → 124 + rescue;
  `--config` honored by default action; repo-local notice printed once; dry-run dedupe loop;
  `[generation]` output/strip applied; manifest `strip_code_fence=false`; manifest `output=json`;
  `[generation] output=json`; `max_duplicate_retries=0` → rescue; empty-output parse-fail → rescue;
  non-zero agent exit + partial stdout accepted; `-a/--all` forces staging; CAS failure (HEAD
  unchanged) via raced commit; `--version`; verbose command + retry; `providers list/show`;
  `config init/path` with HOME/XDG precedence + existing-file guard; merge-conflict abort;
  SIGINT→rescue via the real-binary integration test; dry-run + timeout; empty-file staged;
  **nonexistent `--config`/`STAGECOACH_CONFIG` → real-agent fallback**; directory/malformed
  `--config` → error; `providers show` claims json vs parse raw).
- **Passing**: all happy-path, data-integrity, rescue, timeout, CAS, signal, merge-conflict,
  root-commit, prompt, render, parse, config-precedence, and stage-while-generating scenarios; all
  seven bugfix-001 fixes verified non-regressed.
- **Failing / deviating**: Issue 1 (Major), Issue 2 (Major), Issue 3 (Minor), Issue 4 (Minor).
- **Areas with good coverage**: snapshot/atomic-commit core (§13), git plumbing + exit-code
  semantics, rescue/timeout/CAS/signal error mapping, prompt assembly, provider render + parse,
  config precedence resolution, auto-stage state machine.
- **Areas needing more attention**: the **`[generation]`↔manifest override seam** (Issue 2 — the
  tri-state defaults defeat the "only override when explicitly set" intent), the **explicit-vs-
  discovery config-path distinction** (Issue 1), and **user-facing error wording** for merge
  conflicts (Issue 3) and dry-run failures (Issue 4).
