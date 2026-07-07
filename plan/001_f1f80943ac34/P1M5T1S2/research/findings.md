# P1.M5.T1.S2 Research Findings — Real-agent integration test scaffold

> Research for the `//go:build integration_real` suite (PRD §20.1 layer 4). Manual pre-release smoke
> test that drives `generate.CommitStaged` against each of the 6 REAL provider manifests, gated by the
> `integration_real` build tag AND `STAGECOACH_RUN_REAL=1`. NOT in CI (`make test` = `go test -race ./...`,
> no `-tags`). All findings verified against the live tree on 2026-06-29.

---

## 1. The seam: `generate.CommitStaged` (the orchestrator to drive) — `internal/generate/generate.go`

```go
type Deps struct {
    Git      git.Git           // real git.Git via git.New(repo)
    Manifest provider.Manifest // ← HERE: pass a REAL builtin manifest (not the stub)
    Verbose  *ui.Verbose       // nil-safe; pass nil or ui.NewVerbose(nil,false)
}
func CommitStaged(ctx context.Context, deps Deps, cfg config.Config) (Result, error)
type Result struct {
    CommitSHA, Subject, Message, Provider, Model string
    Changes []git.FileChange
}
```

- `CommitStaged` is EXPORTED (capital C) → callable from any `package generate` test file.
- It calls `deps.Manifest.Render(cfg.Model, cfg.Provider, sysPrompt, payload)` then
  `provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)` then `provider.ParseOutput(out, deps.Manifest)`.
- Passing a real `provider.Manifest` (from `BuiltinManifests`/registry) is EXACTLY what makes this a
  real-agent test: `Render` builds the real argv, `Execute` spawns the real binary, the binary writes a
  real message, `ParseOutput` parses it, and the orchestrator commits. No stub anywhere.

## 2. The 6 manifests + their model/provider defaults — `internal/provider/builtin.go`

`provider.BuiltinManifests()` → `map[string]Manifest{pi, claude, gemini, opencode, codex, cursor}`.
After `Resolve()`, the relevant resolved fields are:

| name     | Detect  | Command | DefaultModel         | DefaultProvider | PromptDelivery | PrintFlag |
|----------|---------|---------|----------------------|-----------------|----------------|-----------|
| pi       | pi      | pi      | glm-5-turbo          | "" (explicit)   | stdin          | -p        |
| claude   | claude  | claude  | sonnet               | n/a ("")        | stdin          | -p        |
| gemini   | gemini  | gemini  | gemini-2.5-pro       | n/a ("")        | stdin          | ""        |
| opencode | opencode| opencode| "" (user MUST set)   | n/a ("")        | positional     | ""        |
| codex    | codex   | codex   | "" (config.toml)     | n/a ("")        | stdin          | ""        |
| cursor   | agent   | agent   | "" (per-account)     | n/a ("")        | positional     | -p        |

**CRITICAL — cursor is the ONLY provider where Detect ≠ Name** (Detect="agent", the binary). The
registry's `IsInstalled` probes `m.DetectCommand()` (= Detect if set else Command), so it correctly
looks for `agent` on $PATH for cursor. Do NOT hardcode `exec.LookPath("cursor")`.

## 3. Model/provider fallback in `Render` — `internal/provider/render.go` (the key design challenge)

```go
modelToUse := model        // cfg.Model
if modelToUse == "" { modelToUse = *r.DefaultModel }    // manifest default
providerToUse := provider  // cfg.Provider
if providerToUse == "" { providerToUse = *r.DefaultProvider }
// emitted only if the flag is non-empty AND the value is non-empty:
if *r.ProviderFlag != "" && providerToUse != "" { args = append(args, *r.ProviderFlag, providerToUse) }
if *r.ModelFlag    != "" && modelToUse    != "" { args = append(args, *r.ModelFlag, modelToUse) }
```

**Implication for the real test (environment-specific models — the headline gotcha):**
- **pi**: manifest `default_model="glm-5-turbo"`, `default_provider=""` (→ no `--provider` emitted) → pi
  runs with its built-in default provider `google`, which does NOT serve `glm-5-turbo`. → the test MUST
  set `cfg.Provider="zai"` for pi (the commit-pi convention; external_deps §pi verified `--provider zai`).
- **opencode**: `default_model=""` → Render emits NO `-m` → `opencode run` may reject (no model). → the
  test sets `cfg.Model="anthropic/claude-sonnet-4"` (external_deps §opencode rendered example).
- **codex**: `default_model=""` → NO `-m` → codex uses `~/.codex/config.toml` (external_deps §codex).
  Leave `cfg.Model=""` (codex is configured via its own config file).
- **cursor**: `default_model=""` → NO `--model` → cursor uses its per-account default. Leave `cfg.Model=""`.

**These are environment-specific.** The test MUST make model+provider env-overridable
(`STAGECOACH_REAL_MODEL_<NAME>` / `STAGECOACH_REAL_PROVIDER_<NAME>`) with documented best-effort
defaults (§4), so the operator can point at whatever model is actually available.

## 4. Documented best-effort model/provider defaults (env-overridable)

```go
var realDefaults = map[string]struct{ model, provider string }{
    "pi":       {"", "zai"},                    // glm-5-turbo comes from the manifest default; provider=zai (commit-pi)
    "claude":   {"", ""},                       // sonnet from manifest default
    "gemini":   {"", ""},                       // gemini-2.5-pro from manifest default
    "opencode": {"anthropic/claude-sonnet-4", ""}, // manifest default is "" → MUST supply
    "codex":    {"", ""},                       // model from ~/.codex/config.toml
    "cursor":   {"", ""},                       // per-account default model
}
// cfg.Model     = envOr("STAGECOACH_REAL_MODEL_"    + UPPER(name), realDefaults[name].model)
// cfg.Provider  = envOr("STAGECOACH_REAL_PROVIDER_" + UPPER(name), realDefaults[name].provider)
```
All six verified present on this machine per `architecture/external_deps.md` (2026-06-29).

## 5. Registry: getting a real manifest + detecting installation — `internal/provider/registry.go`

```go
reg := provider.NewRegistry(nil)          // nil overrides → pure built-ins (no user config noise)
m, ok := reg.Get(name)                    // the merged manifest (built-in ⊕ nothing)
reg.IsInstalled(m)                        // exec.LookPath(m.DetectCommand()); false if not on $PATH
```
`NewRegistry(nil)` is the right entry: it seeds `BuiltinManifests()` fresh and overlays nothing, so the
test exercises the SHIPPED manifests exactly as the CLI resolves them. (Passing user overrides would
test local config, not the built-ins — wrong for a release gate.) `IsInstalled` is the graceful-skip seam.

## 6. The TO CONFIRM items this suite RESOLVES — `external_deps.md` + `builtin.go` comments

Two manifest comments carry `// TO CONFIRM (integration):` notes pointing squarely at this task:
1. **codex** (`builtinCodex`): "that `codex exec` writes the assistant's final answer to stdout and
   exits 0 on success." A passing `TestRealAgents/codex` (err==nil, non-empty message, commit created)
   CONFIRMS the codex manifest (`subcommand=["exec"]`, `prompt_delivery="stdin"`,
   `bare_flags=["--sandbox","read-only","--ephemeral"]`) works end-to-end. Failure → the manifest is
   wrong (e.g. stdout not the right channel → fall back to `-o <file>` / `--json`).
2. **cursor** (`builtinCursor`): "that `--mode ask` wins over `-p`'s default full-tools profile — i.e.
   the combo (-p --mode ask --trust) is genuinely read-only." A passing `TestRealAgents/cursor`
   CONFIRMS it produces a valid one-shot message. (Read-only-ness is also implied: if it weren't
   read-only it would either block on a prompt — failing the timeout — or mutate the temp repo, which
   the commit-message round-trip would not catch but the repo-safety story assumes.)

The suite should LOG the resolved command per subtest (render the spec, truncate the payload) so the
operator can SEE exactly what ran and visually confirm the read-only flags. This is the manual
release gate's whole point.

## 7. Reusable helpers in `generate_test.go` (package generate — REUSE, do NOT redeclare)

`internal/generate/generate_test.go` is `package generate` (internal test) and declares:
`initRepo`, `writeFile`, `stageFile`, `commitRaw` (--allow-empty), `headSHA`, `gitOut`, `runGit`, `shaRe`
(`^[0-9a-f]{7,64}$`). The new file is ALSO `package generate` → it can call these DIRECTLY. Do NOT
redeclare any (compile error: redeclared in this block).

**Coexistence with S1 (`invariants_test.go`, present):** S1 declares its OWN helpers
(`snapshotRepo`, `treeSHAFromErr`, `assertInvariants`, `repoSnapshot`) — distinct names — and reuses
the same `generate_test.go` helpers. The new file must use YET OTHER distinct helper names
(`realConfig`, `envOr`, `logResolvedCommand`) to avoid any collision when `-tags integration_real`
compiles BOTH files together. Confirmed: no existing test func is named `TestRealAgents`.

## 8. Build-tag convention + CI exclusion (the opt-in mechanism) — VERIFIED

- **Build tag**: Go 1.17+ `//go:build` syntax. The file's FIRST line must be `//go:build integration_real`
  followed by a BLANK line, then `package generate` (mirror `internal/provider/procgroup_unix.go`'s
  `//go:build !windows` + blank line + `package provider`).
- **Double gate**: (a) the build tag excludes the file from `go test ./...` / `make test` / `make coverage`
  (Makefile: `go test -race ./...` and `go test -coverprofile=... ./...` — neither passes `-tags`);
  (b) the env var `STAGECOACH_RUN_REAL=1` is a runtime gate so even `go test -tags integration_real ./...`
  without the env var skips (defense-in-depth against accidental slow/costly real runs).
- **Per-subtest gate**: `if !reg.IsInstalled(m) { t.Skipf("%s (%s) not on $PATH", name, m.DetectCommand()) }`.
- **Run command (manual)**: `STAGECOACH_RUN_REAL=1 go test -tags integration_real ./internal/generate/ -run TestRealAgents -v -timeout 30m`.
  (`-timeout 30m`: real agents are slow; Go's test timeout, distinct from `cfg.Timeout`=120s per attempt.)

## 9. Assertions (deterministic on the COMMIT, NOT the message content)

The agent's message is nondeterministic — assert on the COMMIT, not the words:

| assertion | how |
|---|---|
| generation succeeded | `err == nil` (else `t.Fatalf("real agent %s failed: %v", name, err)`) |
| non-empty message | `res.Message != ""` |
| valid commit SHA | `shaRe.MatchString(res.CommitSHA)` |
| HEAD moved | `headSHA(repo) == res.CommitSHA` |
| message round-trips | `gitOut(repo, "log", "--format=%B", "-n1", res.CommitSHA) == res.Message` |
| DiffTree reported change | `len(res.Changes) > 0` |

Fixture (mirror `TestCommitStaged_Success`): `initRepo` → `commitRaw("initial")` → `writeFile` a
real-ish diff (a small Go/markdown file) → `stageFile`. Repo has CommitCount==1 → `buildSystemPrompt`
takes the fallback path (§17.2); dedupe is vacuous (only subject "initial" to avoid, very unlikely
collision) → single attempt. `cfg = config.Defaults()` gives `Timeout=120s` (ample per attempt).

## 10. Validation commands (verified against this tree)

```bash
# Level 1 — format/vet WITH the tag (the file only compiles under the tag):
go test -tags integration_real ./internal/generate/   # compiles the suite (compile check)
gofmt -l internal/generate/                            # empty
go vet -tags integration_real ./internal/generate/     # clean

# Level 2 — env-gate skip (no real agents run, no API cost):
STAGECOACH_RUN_REAL=0 go test -tags integration_real ./internal/generate/ -run TestRealAgents -v
# → every subtest SKIPS ("set STAGECOACH_RUN_REAL=1 to run") OR skips on not-installed.

# Level 3 — the real manual gate (requires all 6 agents + network/API):
STAGECOACH_RUN_REAL=1 go test -tags integration_real ./internal/generate/ -run TestRealAgents -v -timeout 30m

# Level 4 — CI-exclusion proof + no regression:
make test            # go test -race ./... (NO tag → file excluded → no real runs, green)
git status --short   # ONLY ?? internal/generate/realagent_test.go
```

## 11. Gaps / risks

- **Model availability is environment-specific.** A failing `pi`/`opencode`/`cursor` subtest may just
  mean that model isn't provisioned in this env, not that the manifest is broken. Mitigation: env
  overrides (§4) + the logged resolved command (§6) so the operator can distinguish "manifest wrong"
  from "model unavailable". The subtests for codex/cursor are the TO CONFIRM deliverables; the others
  are bonus breadth (all 6 named in the contract).
- **Network/API cost + slowness.** Real agents hit the network and cost money. The double gate
  (build tag + env var) + per-subtest install check + Go `-timeout` bound this. Never run in CI.
- **No new deps.** Everything needed (context, errors, fmt, os, strings, testing, time, config, git,
  provider) is already imported in generate_test.go. Stdlib + existing internal pkgs only.
