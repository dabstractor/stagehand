# S1 Implementation Notes — single-commit Render provider-param fix

> Scope: P1.M1.T1.S1 — fix the provider/sub-provider conflation at the single Render call site in
> `internal/generate/generate.go` (CommitStaged's generate→dedupe loop). Pass `""` instead of
> `cfg.Provider`. Verified against live source on 2026-07-01.

## 1. The single edit locus (internal/generate/generate.go, verbatim current line ~192)

Inside `CommitStaged`'s `for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++` loop:
```go
spec, rerr := deps.Manifest.Render(cfg.Model, cfg.Provider, sysPrompt, payload)
if rerr != nil {
    return Result{}, fmt.Errorf("commit staged: render: %w", rerr)
}
```
The second positional arg `cfg.Provider` is the **manifest/agent NAME** (e.g. "pi"), NOT the
sub-provider. `cfg.Provider` comes from `config.Load` and is always the manifest name or "" (auto).
It is NEVER a valid sub-provider. This is the ONLY use of `cfg.Provider` inside CommitStaged.

## 2. Why `""` is the correct fix (render.go fallback — verbatim, lines 87-104)

```go
r := m.Resolve() // safe `*r.X` deref; copy — caller's m untouched
modelToUse := model
if modelToUse == "" { modelToUse = *r.DefaultModel }
providerToUse := provider
if providerToUse == "" { providerToUse = *r.DefaultProvider }   // <-- the fallback we WANT
...
if *r.ProviderFlag != "" && providerToUse != "" {
    args = append(args, *r.ProviderFlag, providerToUse)          // emits --provider <DefaultProvider>
}
```
- Passing `""` → `providerToUse = *r.DefaultProvider` (the FR37a-merged sub-provider, e.g. "openrouter").
- If `DefaultProvider == ""` (pi's shipped default, §12.3) → `providerToUse == ""` → `--provider` OMITTED.
- `model` param stays `cfg.Model` (Render already falls back to `*r.DefaultModel` when empty — no change).
- render.go's fallback logic is ALREADY CORRECT — S1 does NOT touch render.go. The bug is purely the
  caller passing the wrong value. The architecture analysis (issue1_provider_conflation.md) confirms:
  "No changes needed to render.go."

## 3. The fix is a one-token change + an explanatory comment

```go
// OLD:
spec, rerr := deps.Manifest.Render(cfg.Model, cfg.Provider, sysPrompt, payload)
// NEW:
// Pass "" for the sub-provider: cfg.Provider is the manifest/agent NAME (registry key, e.g. "pi"),
// NOT the upstream backend. Render resolves the real sub-provider from the manifest's merged
// DefaultProvider (FR37a) — emitting "--provider <DefaultProvider>" or omitting it when unset.
spec, rerr := deps.Manifest.Render(cfg.Model, "", sysPrompt, payload)
```
No other change to generate.go. Result struct unchanged. `Result.Provider` (line 292) stays
`deps.Manifest.Name` (correct — it's the agent name, not the sub-provider).

## 4. The verbose-capture chain for the test (verified)

- `provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)` (generate.go:200) → executor.go:63:
  `vb.VerboseCommand(strings.Join(append([]string{spec.Command}, spec.Args...), " "))`.
- `VerboseCommand` (ui/verbose.go:40) writes `"DEBUG: command: " + cmd + "\n"` to `v.w` when
  `v != nil && v.w != nil && v.on`. It is nil-safe (existing tests pass nil Verbose → no-op).
- So `deps.Verbose = ui.NewVerbose(&buf, true)` captures the FULL rendered argv (Command+Args) in buf.
- SECURITY note: VerboseCommand logs ARGV only, never Env — so no key leakage. Safe to assert on.

## 5. Test design (generate_test.go) — pi-shaped stub, capture argv, assert the sub-provider

stubtest.Manifest(bin, Options) returns a `provider.Manifest` with Name="stub", Command=bin,
PromptDelivery="stdin", Output="raw", StripCodeFence=true, Env=knobs. It does NOT set
ProviderFlag/DefaultProvider/ModelFlag/etc. To make it pi-shaped, set pointer fields via LOCAL
addresses (provider.strPtr is unexported; cross-package can't call it — but fields are exported):

```go
import (
    "bytes"
    "github.com/dustin/stagehand/internal/ui"
)
...
bin := stubtest.Build(t)
repo := t.TempDir(); initRepo(t, repo); commitRaw(t, repo, "initial")
writeFile(t, repo, "f.txt", "x"); stageFile(t, repo, "f.txt")

m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: provider ok"})
pflag, dp := "--provider", "openrouter"
m.ProviderFlag = &pflag      // pi-shaped: emit --provider
m.DefaultProvider = &dp      // the merged sub-provider that MUST be honored

cfg := config.Defaults()
cfg.Provider = "pi"          // the manifest NAME — the conflation source; must NOT be emitted

var buf bytes.Buffer
deps := Deps{Git: git.New(repo), Manifest: m, Verbose: ui.NewVerbose(&buf, true)}
res, err := CommitStaged(context.Background(), deps, cfg)
// assert: err == nil; res.Subject == "feat: provider ok"
// assert: strings.Contains(buf.String(), "--provider openrouter")
// assert: !strings.Contains(buf.String(), "--provider pi")   // <-- the bug's signature
```

Why this is faithful: `cfg.Provider = "pi"` is exactly what the bootstrap config / `--provider pi` /
`git config stagehand.provider pi` produce. Before the fix, Render("pi") → `--provider pi` (the
bug). After the fix, Render("") → falls back to DefaultProvider="openrouter" → `--provider openrouter`.
The negative assertion `!strings.Contains(buf, "--provider pi")` is the direct regression guard.

Back-compat (existing tests): TestCommitStaged_Success et al. use `Deps{Git:..., Manifest: m}`
WITHOUT Verbose → `deps.Verbose == nil` → VerboseCommand no-op → they still pass unchanged. The
stubtest.Manifest default has DefaultProvider==nil → after fix Render("") → `*r.DefaultProvider==""`
→ `--provider` omitted → byte-identical to before (before the fix, cfg.Provider was "" in Defaults,
so Render("") anyway). NO existing test changes.

## 6. Scope discipline — what S1 does NOT do

- NOT internal/decompose/{planner,stager,message,arbiter}.go — those four Render call sites are S2
  (P1.M1.T1.S2). They have the SAME bug but are a separate subtask.
- NOT render.go, config/roles.go (ResolveRoleModel), provider/merge.go — all already correct; the
  architecture analysis confirms no changes there.
- NOT the E2E CLI integration test (P1.M1.T2.S1) — that drives the real binary via a config file.
  S1 adds only the caller-level unit test in generate_test.go.
- NOT PRD.md / tasks.json / prd_snapshot.md / plan/*.
- DOCS: inline comment on the Render call only (Mode A) — no docs/*.md, no separate docs subtask.
