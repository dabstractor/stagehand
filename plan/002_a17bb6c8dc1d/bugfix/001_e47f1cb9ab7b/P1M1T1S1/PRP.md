---
name: "P1.M1.T1.S1 — Fix single-commit Render call in generate.go (provider/sub-provider conflation)"
description: |
  Bugfix-001 Issue 1 (Critical). At `internal/generate/generate.go` ~L192 inside `CommitStaged`'s
  generate→dedupe loop, `deps.Manifest.Render(cfg.Model, cfg.Provider, sysPrompt, payload)` passes
  `cfg.Provider` — which is the manifest/agent NAME ("pi"), NOT the upstream sub-provider. Render then
  emits `--provider pi` and silently ignores the user's configured `default_provider`. Fix: pass `""`
  so Render falls back to the manifest's merged `DefaultProvider` (FR37a), emitting
  `--provider <sub-provider>` or omitting the flag when unset. One-token edit + explanatory comment +
  one caller-level unit test in generate_test.go (pi-shaped stub + verbose-buffer argv capture).
  Decompose role fixes (S2), render.go/merge.go/roles.go (no change), and the E2E CLI test (T2) are
  OUT OF SCOPE.
---

## Goal

**Feature Goal**: Eliminate the provider/sub-provider conflation at the single-commit Render call site
so that `CommitStaged` renders the **sub-provider** (`zai`, `openrouter`, …) resolved from the
manifest's merged `DefaultProvider` (FR37a) instead of the manifest/agent name (`pi`). With the fix,
`stagecoach` (single-commit path) emits `pi --provider openrouter …` when `default_provider` is set, or
omits `--provider` entirely when it is not — exactly per PRD §12.2/§12.3. This is the root-cause fix
for the Critical Issue 1 (the default provider, pi, is broken in every common configuration).

**Deliverable** (ONE production file + its test file):
1. `internal/generate/generate.go` ~L192: change the Render call's provider argument from `cfg.Provider`
   to `""`, with an inline comment explaining why (cfg.Provider is the manifest name / registry key,
   NOT the sub-provider; Render resolves the sub-provider from the manifest's `DefaultProvider`).
2. `internal/generate/generate_test.go`: add `TestCommitStaged_ResolvesSubProviderFromManifest` — a
   pi-shaped stub manifest with `DefaultProvider="openrouter"` + `ProviderFlag="--provider"`,
   `cfg.Provider="pi"` (the conflation source), capturing the rendered argv via `deps.Verbose`, asserting
   the command contains `--provider openrouter` and does NOT contain `--provider pi`.

**Success Definition**: `CommitStaged` passes `""` (not `cfg.Provider`) to Render; the rendered command
emits the manifest's `DefaultProvider` as `--provider <value>` when set, or omits `--provider` when
unset; the `Result` struct and all other generate.go logic are unchanged; existing tests pass
unchanged (stubtest default has no `DefaultProvider` → `--provider` omitted, byte-identical to before);
`go build/vet/gofmt` clean and `go test -race ./...` green.

## User Persona

**Target User**: Every Stagecoach user who configures the default provider (pi) — the most common
configuration. Specifically: the bootstrap config (`[defaults] provider = "pi"`), `--provider pi`,
`git config stagecoach.provider pi` (PRD §15.5's recommended setup), and `STAGECOACH_PROVIDER=pi`.

**Use Case**: A user sets `default_provider = "openrouter"` under `[provider.pi]` so pi routes to an
OpenRouter backend, then runs `stagecoach` to generate a commit.

**Pain Points Addressed**: Today the rendered command is `pi --provider pi …` — an invalid sub-provider
that overrides and silently ignores the user's configured `default_provider`, defeating the FR37a merge
fix that was supposed to preserve it. The fix makes the documented `--provider <backend>` honoring work.

## Why

- **Critical, default-path bug.** This breaks pi — the shipped default provider — in EVERY common
  configuration (bootstrap, `--provider pi`, `git config stagecoach.provider pi`, env). It is Issue 1,
  the single Critical issue in the v2.0 QA pass.
- **Defeats an already-correct layer.** FR37a's field-merge correctly preserves `default_provider`
  across config layers into the manifest's `DefaultProvider`. That value is correct at the merge layer
  — but Render never reads it because the caller overrides it with the manifest name. The fix lets the
  already-correct merge actually take effect.
- **render.go is already correct.** Render's `provider == "" → *r.DefaultProvider` fallback is right;
  the bug is purely that the caller passes the wrong (non-empty) value. So the fix is a one-token change
  at the call site, not a redesign. (Verified in `issue1_provider_conflation.md`: "No changes needed to
  render.go / config/roles.go / provider/merge.go.")
- **Minimal, surgical, back-compatible.** Existing tests pass `cfg := config.Defaults()` (Provider=""),
  so they already hit Render("") and are byte-identical after the fix. The change touches one token.

## What

A one-token edit (`cfg.Provider` → `""`) plus an inline comment at `generate.go:192`, and one new
caller-level unit test in `generate_test.go`. No changes to render.go, config/roles.go, merge.go, the
Result struct, or any decompose file.

### Success Criteria

- [ ] `generate.go` ~L192 calls `deps.Manifest.Render(cfg.Model, "", sysPrompt, payload)` (not `cfg.Provider`).
- [ ] The call has an inline comment explaining `cfg.Provider` is the manifest name, not the sub-provider.
- [ ] `CommitStaged`, with a manifest whose `DefaultProvider="openrouter"` + `ProviderFlag="--provider"`
      and `cfg.Provider="pi"`, renders a command containing `--provider openrouter`.
- [ ] That command does NOT contain `--provider pi` (the manifest-name conflation signature).
- [ ] With no `DefaultProvider` set, `--provider` is omitted (pi's shipped default, §12.3).
- [ ] `Result` struct unchanged; `Result.Provider` still = `deps.Manifest.Name`.
- [ ] Existing generate tests pass unchanged (no edits to them).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.
- [ ] No edits to render.go, config/roles.go, provider/merge.go, or any `internal/decompose/*.go`.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the exact buggy line, the exact Render fallback logic (verbatim)
that makes `""` correct, the exact verbose-capture chain (Execute → VerboseCommand → buffer), the
complete test (imports, manifest construction via local-address pointer fields, assertions), and the
executable validation commands. The architecture analysis (`issue1_provider_conflation.md`) pre-resolved
root cause, the render.go-already-correct finding, and the scope boundary vs S2/T2.

### Documentation & References

```yaml
# MUST READ — the binding issue analysis (do not re-litigate)
- docfile: plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/architecture/issue1_provider_conflation.md
  why: "Pinpoints the exact call site (generate.go ~L192), quotes the Render fallback that makes '' correct, lists all 5 call sites (this subtask = generate.go ONLY; the 4 decompose sites = S2), and confirms render.go/roles.go/merge.go need NO changes. Gives the test strategy (caller-level unit test asserting the rendered argv)."
  critical: "States render.go's fallback is already correct and the bug is purely the caller passing the manifest name. States the fix is 'Render(cfg.Model, \"\", ...)' with NO other generate.go changes. S1 is generate.go + its test ONLY; decompose roles are S2 (P1.M1.T1.S2); the E2E CLI test is T2 (P1.M1.T2.S1)."

# The single production file under edit
- file: internal/generate/generate.go
  why: "THE edit target. CommitStaged's generate→dedupe loop, ~L192: `deps.Manifest.Render(cfg.Model, cfg.Provider, sysPrompt, payload)`. cfg.Provider is the ONLY use of cfg.Provider in CommitStaged. Result.Provider (L292) = deps.Manifest.Name (unchanged — that IS the agent name)."
  pattern: "Change the 2nd positional Render arg from cfg.Provider to \"\"; add the explanatory comment. Resolve() is already called at L175 (resolved := deps.Manifest.Resolve()) but Render does its own Resolve internally (render.go:87) — both read the manifest's DefaultProvider."
  gotcha: "Do NOT change the model arg (cfg.Model) — Render already falls back to *r.DefaultModel when model is empty. Do NOT change Result.Provider (deps.Manifest.Name is correct). Do NOT touch any other line in generate.go."

# Cross-references (read-only — do NOT edit in S1)
- file: internal/provider/render.go
  why: "Render's provider fallback (lines 94-96, 102-104): `providerToUse := provider; if providerToUse == \"\" { providerToUse = *r.DefaultProvider }; if *r.ProviderFlag != \"\" && providerToUse != \"\" { args = append(args, *r.ProviderFlag, providerToUse) }`. Confirms passing \"\" emits DefaultProvider or omits --provider. NOT edited."
  gotcha: "Render internally calls m.Resolve() (L87) — so the manifest's DefaultProvider/ProviderFlag (set on deps.Manifest) are read from the manifest you pass, regardless of the L175 resolved var."

- file: internal/provider/executor.go
  why: "Execute(ctx, *spec, cfg.Timeout, deps.Verbose) at L44; L63 calls vb.VerboseCommand(strings.Join(append([]string{spec.Command}, spec.Args...), \" \")). Confirms the verbose sink captures the FULL rendered argv (Command+Args) — the assertion target. NOT edited."

- file: internal/ui/verbose.go
  why: "NewVerbose(w io.Writer, on bool) *Verbose (L33); VerboseCommand(cmd string) writes \"DEBUG: command: \"+cmd+\"\\n\" when on && w!=nil, nil-safe (L40). Confirms the test-capture pattern `ui.NewVerbose(&buf, true)`. NOT edited."

- file: internal/stubtest/stubtest.go
  why: "Manifest(bin, Options) returns a provider.Manifest (Name=\"stub\", Command=bin, PromptDelivery=\"stdin\", Output=\"raw\", StripCodeFence=true, Env=knobs). Does NOT set ProviderFlag/DefaultProvider/ModelFlag. To make it pi-shaped, SET those fields on the returned manifest via local-address pointers (provider.strPtr is unexported; cross-package can't call it, but the fields are exported)."

- file: internal/generate/generate_test.go
  why: "EDIT TARGET (test). Reuses helpers: stubtest.Build(t), initRepo/writeFile/stageFile/commitRaw/runGit/headSHA. Existing tests construct `Deps{Git: git.New(repo), Manifest: m}` (no Verbose → nil → no-op). TestCommitStaged_Success (:79) is the style template. NEW test adds imports: bytes + internal/ui."

- docfile: plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/P1M1T1S1/research/s1_implementation_notes.md
  why: "Distilled S1 findings: the exact edit, the verbatim Render fallback, the verbose-capture chain, the complete test (with the local-address pointer-field trick), back-compat proof, and the S1-vs-S2-vs-T2 scope boundary."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
├── internal/generate/
│   ├── generate.go        # EDIT TARGET (CommitStaged Render call ~L192)
│   └── generate_test.go   # EDIT TARGET (add TestCommitStaged_ResolvesSubProviderFromManifest)
├── internal/provider/
│   ├── render.go          # read-only ref — Render fallback ALREADY CORRECT (no edit)
│   └── executor.go        # read-only ref — Execute→VerboseCommand argv capture
├── internal/ui/verbose.go # read-only ref — NewVerbose + VerboseCommand (nil-safe)
├── internal/stubtest/stubtest.go  # read-only ref — Manifest(bin, Options) helper
└── internal/decompose/    # NOT touched in S1 (the 4 role Render sites are S2)
```

### Desired Codebase Tree After S1

```bash
stagecoach/
└── (only existing files modified — no new files)
    internal/generate/generate.go        # Render call: cfg.Provider → ""  (+ comment)
    internal/generate/generate_test.go   # +1 test (TestCommitStaged_ResolvesSubProviderFromManifest) + 2 imports
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/generate/generate.go` | MODIFY | Pass `""` (not `cfg.Provider`) to Render at ~L192; add explanatory comment. **Only production file touched.** |
| `internal/generate/generate_test.go` | MODIFY | Add a pi-shaped-stub test asserting `--provider openrouter` (not `--provider pi`); add `bytes` + `internal/ui` imports. |

**Explicitly NOT touched**: `internal/decompose/{planner,stager,message,arbiter}.go` (S2),
`internal/provider/render.go` (already correct), `internal/provider/merge.go` (FR37a already correct),
`internal/config/roles.go` (ResolveRoleModel returns the manifest name correctly for `reg.Get`; only
decompose callers misuse it — S2's concern), the `Result` struct, any `docs/*.md` (Mode A = inline
comment only), `PRD.md`, `tasks.json`, `prd_snapshot.md`, anything under `plan/`.

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL: cfg.Provider is the manifest/agent NAME ("pi"), the registry key — NOT the sub-provider.
// Render's 2nd positional param IS the sub-provider (zai/openrouter/...). Passing cfg.Provider makes
// Render emit "--provider pi" (an invalid sub-provider) and silently ignore the merged DefaultProvider.
// The fix passes "" so Render falls back to *r.DefaultProvider (render.go:94-96).

// CRITICAL: the fix is ONE token. Do NOT "also" change the model arg (cfg.Model) — Render already
// falls back to *r.DefaultModel when model=="". Do NOT change Result.Provider (deps.Manifest.Name is
// correct — Result.Provider reports the AGENT, not the sub-provider).

// GOTCHA (cross-package pointer fields): provider.strPtr/boolPtr are UNEXPORTED, so a test in package
// `generate` cannot call them. But Manifest's fields are EXPORTED (*string/*bool). Set them by taking
// the address of a LOCAL variable: `pflag, dp := "--provider", "openrouter"; m.ProviderFlag = &pflag;
// m.DefaultProvider = &dp`. Do NOT take & of a loop var or a struct-literal field that escapes oddly —
// a plain local is the safe, idiomatic pattern (matches how callers build overrides).

// GOTCHA (verbose capture): VerboseCommand joins [Command]+Args with spaces. So
// strings.Contains(buf.String(), "--provider openrouter") matches the emitted token pair. The NEGATIVE
// assertion !strings.Contains(buf.String(), "--provider pi") is the direct regression guard: before
// the fix the buffer WOULD contain "--provider pi" (cfg.Provider="pi" passed through).

// GOTCHA (back-compat): existing tests use cfg := config.Defaults() (Provider="") and no Verbose.
// Before the fix they already passed Render("") (cfg.Provider was ""). After the fix they still pass
// Render(""). byte-identical → NO existing test changes. Do not modify them.

// GOTCHA (Resolve vs Render): CommitStaged calls resolved := deps.Manifest.Resolve() at L175, but
// Render ALSO calls m.Resolve() internally (render.go:87). Both read DefaultProvider from the SAME
// deps.Manifest you pass. So set ProviderFlag/DefaultProvider on deps.Manifest (the m you build), not
// on a separate Resolve() copy.

// SECURITY (PRD §19): VerboseCommand logs ARGV ONLY (Command+Args), NEVER spec.Env (which carries
// *_API_KEY). Asserting on the buffer is safe — no credential leakage. (ui/verbose.go doc comment.)
```

## Implementation Blueprint

### Data models and structure

No data-model changes. `Deps` (already has a `Verbose *ui.Verbose` field — used at generate.go:200)
and `config.Config` are unchanged. The relevant existing types/signatures (verbatim):

```go
// internal/generate/generate.go (EXISTING — unchanged by S1)
type Deps struct {
    Git      git.Git
    Manifest provider.Manifest
    Verbose  *ui.Verbose   // <-- set this in the new test to capture the rendered argv
}
func CommitStaged(ctx context.Context, deps Deps, cfg config.Config) (Result, error)

// internal/provider/render.go (EXISTING — unchanged by S1; the fallback that makes "" correct)
func (m Manifest) Render(model, provider, sysPrompt, userPayload string, mode ...RenderMode) (*CmdSpec, error)
//   providerToUse := provider; if providerToUse == "" { providerToUse = *r.DefaultProvider }

// internal/ui/verbose.go (EXISTING — unchanged; the capture sink)
func NewVerbose(w io.Writer, on bool) *Verbose
func (v *Verbose) VerboseCommand(cmd string) // writes "DEBUG: command: "+cmd+"\n" when on && w!=nil
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/generate/generate.go — pass "" to Render + comment
  - LOCATE: CommitStaged's generate→dedupe loop, the line:
        spec, rerr := deps.Manifest.Render(cfg.Model, cfg.Provider, sysPrompt, payload)
  - REPLACE with (comment + one-token change):
        // Pass "" for the sub-provider: cfg.Provider is the manifest/agent NAME (the registry key,
        // e.g. "pi"), NOT the upstream backend. Render resolves the real sub-provider from the
        // manifest's merged DefaultProvider (FR37a) — emitting "--provider <DefaultProvider>", or
        // omitting --provider when DefaultProvider is unset (pi's shipped default, §12.3).
        spec, rerr := deps.Manifest.Render(cfg.Model, "", sysPrompt, payload)
  - PRESERVE the immediately-following `if rerr != nil { return Result{}, fmt.Errorf("commit staged: render: %w", rerr) }`.
  - DO NOT: change the model arg (cfg.Model), Result.Provider, or any other line.
  - VERIFY: `go build ./internal/generate/` compiles.

Task 2: ADD TestCommitStaged_ResolvesSubProviderFromManifest in internal/generate/generate_test.go
  - ADD imports (if not already present): "bytes" and "github.com/dustin/stagecoach/internal/ui".
  - NAMING: TestCommitStaged_ResolvesSubProviderFromManifest (descriptive; matches existing TestCommitStaged_* family).
  - PLACEMENT: alongside the existing TestCommitStaged_* tests.
  - BODY (reuse helpers initRepo/commitRaw/writeFile/stageFile/runGit/stubtest.Build):
        bin := stubtest.Build(t)
        repo := t.TempDir()
        initRepo(t, repo)
        commitRaw(t, repo, "initial")
        writeFile(t, repo, "f.txt", "content")
        stageFile(t, repo, "f.txt")

        // Pi-shaped stub: emit --provider, with a merged DefaultProvider that MUST be honored.
        m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: provider ok"})
        pflag, dp := "--provider", "openrouter"
        m.ProviderFlag = &pflag
        m.DefaultProvider = &dp

        cfg := config.Defaults()
        cfg.Provider = "pi"   // the manifest NAME — the conflation source; must NOT be emitted

        var buf bytes.Buffer
        deps := Deps{Git: git.New(repo), Manifest: m, Verbose: ui.NewVerbose(&buf, true)}

        res, err := CommitStaged(context.Background(), deps, cfg)
        if err != nil { t.Fatalf("CommitStaged: %v", err) }
        if res.Subject != "feat: provider ok" { t.Errorf("Subject = %q", res.Subject) }

        cmd := buf.String()
        if !strings.Contains(cmd, "--provider openrouter") {
            t.Errorf("rendered command missing --provider openrouter\ngot: %s", cmd)
        }
        if strings.Contains(cmd, "--provider pi") {
            t.Errorf("rendered command emits the manifest name as sub-provider (conflation bug)\ngot: %s", cmd)
        }
  - GOTCHA: take & of LOCALS (pflag, dp), not loop vars. provider.strPtr is unexported — can't use it.
  - COVERAGE: positive (--provider openrouter present) + negative (--provider pi absent) + success path.
  - DO NOT: modify existing tests. DO NOT add a decompose-role test (S2). DO NOT add an E2E CLI test (T2).

Task 3: VALIDATE (run all gates; all must pass before declaring done)
  - RUN: go build ./...
  - RUN: go vet ./...
  - RUN: gofmt -l .            # must be empty
  - RUN: go test -race ./...   # new test green; existing generate tests + full suite green
  - FIX-FORWARD: read failures, fix, re-run. Do NOT skip.
```

### Implementation Patterns & Key Details

```go
// === generate.go — the edited Render call (Task 1, in loop context) ===

	for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++ {
		payload := prompt.BuildUserPayload(diff, rejected)
		if parseFail {
			payload = retryInstr + "\n\n" + payload // FR29 corrective preamble
		}

		// Pass "" for the sub-provider: cfg.Provider is the manifest/agent NAME (the registry key,
		// e.g. "pi"), NOT the upstream backend. Render resolves the real sub-provider from the
		// manifest's merged DefaultProvider (FR37a) — emitting "--provider <DefaultProvider>", or
		// omitting --provider when DefaultProvider is unset (pi's shipped default, §12.3).
		spec, rerr := deps.Manifest.Render(cfg.Model, "", sysPrompt, payload)
		if rerr != nil {
			return Result{}, fmt.Errorf("commit staged: render: %w", rerr)
		}
		// ... Execute / ParseOutput / dedupe unchanged ...
	}
```

```go
// === The Render fallback that makes "" correct (render.go:87-104 — UNCHANGED, for reference) ===

	r := m.Resolve()
	modelToUse := model
	if modelToUse == "" { modelToUse = *r.DefaultModel }
	providerToUse := provider
	if providerToUse == "" { providerToUse = *r.DefaultProvider }   // <-- fires now that we pass ""
	...
	if *r.ProviderFlag != "" && providerToUse != "" {
		args = append(args, *r.ProviderFlag, providerToUse)          // → "--provider openrouter"
	}
```

```go
// === The verbose-capture chain (for the test assertion — UNCHANGED, for reference) ===

// executor.go:63
vb.VerboseCommand(strings.Join(append([]string{spec.Command}, spec.Args...), " "))
// ui/verbose.go:44  →  fmt.Fprintln(v.w, "DEBUG: command: "+cmd)
// buf now holds: "DEBUG: command: <bin> --provider openrouter\n"
```

### Integration Points

```yaml
PRODUCTION (internal/generate/generate.go CommitStaged):
  - Render call 2nd arg: cfg.Provider → "" (one token)
  - inline comment: explains cfg.Provider is the manifest name, not the sub-provider

NO-TOUCH (explicitly — render.go already correct; other call sites are separate subtasks):
  - internal/provider/render.go        # the provider=="" → *r.DefaultProvider fallback is ALREADY correct
  - internal/provider/merge.go         # FR37a default_provider merge is ALREADY correct
  - internal/config/roles.go           # ResolveRoleModel returns the manifest name for reg.Get (correct)
  - internal/decompose/{planner,stager,message,arbiter}.go   # the SAME bug, but = S2 (P1.M1.T1.S2)
  - internal/cmd/*                     # CLI unchanged
  - Result struct                      # unchanged; Result.Provider = deps.Manifest.Name (the agent)

TEST (internal/generate/generate_test.go):
  - +1 test: TestCommitStaged_ResolvesSubProviderFromManifest (pi-shaped stub + verbose capture)
  - +2 imports: bytes, internal/ui

DOCS (Mode A — rides with the work):
  - inline comment on the Render call in generate.go (Task 1). No docs/*.md, no separate docs subtask.

DOWNSTREAM HOOKS (informational — owned by OTHER subtasks, NOT S1):
  - S2 (P1.M1.T1.S2): fix the SAME conflation in the 4 decompose role files (planner/stager/message/arbiter)
  - T2 (P1.M1.T2.S1): E2E CLI integration test driving the real binary with a pi-shaped stubagent config
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -l .                       # Expected: empty (run `gofmt -w internal/generate/generate.go generate_test.go` if listed)
go vet ./internal/generate/...   # Expected: exit 0
go build ./...                   # Expected: exit 0 (render.go/merge.go/decompose all compile unchanged)

# Expected: Zero output/errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/stagecoach

# The new test (asserts the sub-provider is resolved from the manifest, not cfg.Provider)
go test -race -run 'TestCommitStaged_ResolvesSubProviderFromManifest' ./internal/generate/ -v

# The existing generate suite MUST still pass unchanged (they use cfg.Defaults() Provider="" + no Verbose)
go test -race ./internal/generate/ -v

# Expected: new test PASS (buf contains "--provider openrouter", NOT "--provider pi"); all existing
# TestCommitStaged_* pass unchanged.
```

### Level 3: Whole-Repository Regression (No Behavior Change Elsewhere)

```bash
cd /home/dustin/projects/stagecoach

go test -race ./...              # Expected: ALL packages pass
go vet ./...                     # Expected: exit 0

# Confirm ONLY internal/generate/ changed in production source
git diff --stat -- internal/ pkg/ cmd/
# Expected: only internal/generate/generate.go + internal/generate/generate_test.go appear.

# Confirm render.go / merge.go / roles.go / decompose were NOT touched
git diff --name-only -- internal/provider/render.go internal/provider/merge.go internal/config/roles.go internal/decompose/
# Expected: (empty — no output)
```

### Level 4: Bug-Reproduction Cross-Check (manual smoke — optional for S1)

> S1 fixes the single-commit LIBRARY path. The user-visible CLI end-to-end (`stagecoach --dry-run
> --verbose`) also exercises this path, but the dedicated E2E test is T2 (P1.M1.T2.S1). This smoke
> confirms the fix holds through the real binary for the single-commit case.

```bash
cd /home/dustin/projects/stagecoach
go build -o bin/stagecoach ./cmd/stagecoach && go build -o bin/stubagent ./cmd/stubagent

# Throwaway repo + pi-shaped stub config (mirrors the PRD repro)
cd /tmp && rm -rf repro && mkdir repro && cd repro
git init -q && git config user.email t@t.com && git config user.name t && git commit -q --allow-empty -m init
echo x > f.txt && git add f.txt
SH=/home/dustin/projects/stagecoach/bin/stagecoach; STUB=/home/dustin/projects/stagecoach/bin/stubagent
cat > config.toml <<EOF
config_version = 2
[defaults]
provider = "pi"
[provider.pi]
command = "$STUB"
detect  = "$STUB"
provider_flag = "--provider"
default_provider = "openrouter"
model_flag = "--model"
default_model = "gpt-5.4-nano"
system_prompt_flag = "--system"
prompt_delivery = "stdin"
print_flag = "-p"
output = "raw"
[provider.pi.env]
STAGECOACH_STUB_OUT = "feat: repro"
EOF
STAGECOACH_CONFIG=config.toml $SH --dry-run --verbose --no-color 2>&1 | grep "DEBUG: command"
# Expected (after fix): contains "--provider openrouter"; does NOT contain "--provider pi".
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages pass (new generate test green; existing tests unchanged).

### Feature Validation

- [ ] `generate.go` ~L192 calls `Render(cfg.Model, "", ...)` (not `cfg.Provider`).
- [ ] The call carries an inline comment explaining the manifest-name vs sub-provider distinction.
- [ ] New test: pi-shaped stub (DefaultProvider="openrouter", ProviderFlag="--provider") + cfg.Provider="pi"
      → rendered argv contains `--provider openrouter`.
- [ ] New test: rendered argv does NOT contain `--provider pi` (the conflation signature).
- [ ] With no DefaultProvider, `--provider` is omitted (pi shipped default).
- [ ] `Result` struct unchanged; `Result.Provider` == `deps.Manifest.Name`.

### Scope Discipline Validation

- [ ] ONLY `internal/generate/generate.go` (+ `generate_test.go`) modified (git diff --stat confirms).
- [ ] Did NOT edit `render.go`, `merge.go`, `config/roles.go` (all already correct).
- [ ] Did NOT edit `internal/decompose/*.go` (the 4 role sites are S2 / P1.M1.T1.S2).
- [ ] Did NOT add the E2E CLI integration test (that is T2 / P1.M1.T2.S1).
- [ ] Did NOT modify any `docs/*.md` (Mode A = inline comment only).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] Comment explains the WHY (manifest name vs sub-provider), not just the WHAT.
- [ ] Test reuses existing helpers (stubtest.Build, initRepo, writeFile, stageFile, runGit) + family naming.
- [ ] Negative assertion (`!strings.Contains(..., "--provider pi")`) is the direct regression guard.
- [ ] Pointer fields set via local-address (not the unexported provider.strPtr).

---

## Anti-Patterns to Avoid

- ❌ Don't "fix" render.go — its `provider == "" → *r.DefaultProvider` fallback is ALREADY correct. The
  bug is the caller passing the manifest name; the fix is at the call site only. (issue1 analysis confirms.)
- ❌ Don't change the model arg (`cfg.Model`) — Render already falls back to `*r.DefaultModel`. Only the
  provider arg is wrong.
- ❌ Don't change `Result.Provider` (`deps.Manifest.Name`) — that field correctly reports the AGENT, not
  the sub-provider. The user/UI wants to know which agent ran, not which backend.
- ❌ Don't thread a real sub-provider through config in this subtask — that's a larger redesign. Passing
  `""` to let Render resolve from the merged manifest is the minimal, correct, contract-specified fix.
- ❌ Don't fix the 4 decompose role files here (planner/stager/message/arbiter) — they have the SAME bug
  but are S2 (P1.M1.T1.S2). Touching them now crosses the subtask boundary.
- ❌ Don't use the unexported `provider.strPtr` in the test (cross-package can't). Set pointer fields via
  `&localVar` — the fields are exported.
- ❌ Don't forget the negative assertion — `strings.Contains(buf, "--provider openrouter")` alone would
  pass even on a buggy build that ALSO emits `--provider pi`. The `!strings.Contains(buf, "--provider pi")`
  check is the real regression guard.
- ❌ Don't modify existing tests to add Verbose — they pass nil Verbose (no-op) and rely on
  `cfg.Defaults()` (Provider=""), which already hit Render(""). They are byte-identical after the fix.
- ❌ Don't add docs/*.md changes — Mode A is an inline comment on the Render call, nothing more.

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: This is a one-token edit (`cfg.Provider` → `""`) at a single, located line, with the
correctness proven by quoting Render's verbatim fallback logic (passing `""` → `*r.DefaultProvider`).
The architecture analysis (`issue1_provider_conflation.md`) pre-resolved that render.go/merge.go/roles.go
need NO changes and that the fix is purely the call-site argument. The test is fully specified —
including the non-obvious cross-package pointer-field trick (local-address, since provider.strPtr is
unexported), the verbose-capture chain (Execute→VerboseCommand→buffer, verified line-by-line), and the
critical negative assertion. Back-compat is guaranteed (existing tests use Provider="" + nil Verbose).
The only residual uncertainty (not 10/10) is whether the implementer adds the `bytes`/`internal/ui`
imports correctly — a compile error caught immediately by `go build`. The S2 (decompose roles) and T2
(E2E) boundaries are cleanly fenced and cannot be broken by S1.
