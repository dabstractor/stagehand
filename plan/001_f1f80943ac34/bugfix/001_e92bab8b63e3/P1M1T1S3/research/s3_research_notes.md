# S3 Research Notes — Regression tests for `--config` honored (Issue 1) + §19 notice once (Issue 5)

Scope: test-only. S1 (`pkg/stagecoach.Options.Config` + `resolveConfig` skip-Load branch) and S2
(`runDefault` wires `Config: cfg`) are **confirmed landed** in source:

- `pkg/stagecoach/stagecoach.go:126` → `if opts.Config != nil {` (S1 branch).
- `internal/cmd/default_action.go:131` → `Config:    cfg,` (S2 wiring); the comment above it was
  rewritten by S2 to describe the single-Load injection path.

So the two regression tests run **green** against the current tree (they lock in the S1/S2 fix).

## 1. The test seam & harness (confirmed)

- `internal/cmd/default_action_test.go` drives the **full seam** via `Execute(context.Background())` +
  `rootCmd.SetArgs(...)`. Helpers used: `saveRootState`/`restoreRootState` (root_test.go:105-130,
  reset flags + io + loadedCfg between runs), `initRepo`, `chdir`, `writeConfigFile`,
  `writeFile`/`stageFile`/`runGit`/`gitOut`/`headSHA`, `stubtest.Build(t)`.
- `setupStubRepo` writes a `.stagecoach.toml` defining **only** `[provider.stub]` (no top-level
  `provider =`), so the §19 notice does **not** fire in any existing test. No existing test passes
  `--config`, and none asserts the notice count. (Matches seam_config_handoff.md §8.)

## 2. How to capture the §19 notice from package `cmd` (THE crux)

The notice is emitted by `loadRepoLocalConfig` (`internal/config/file.go:226-235`) via
`fmt.Fprint(noticeOut, msg)`, where `noticeOut` is the **unexported** package-level
`var noticeOut io.Writer = os.Stderr` (file.go:50). Consequences:

- `rootCmd.SetErr(&buf)` does **NOT** capture it — `runDefault`'s stderr (`cmd.ErrOrStderr()`) and
  cobra output both go to the SetErr sink, but the notice bypasses the cobra writer entirely.
- `noticeOut` is captured-by-value at package init; reassigning `os.Stderr` later does **not**
  redirect it.
- In-package `config` tests swap it directly (`noticeOut = &strings.Builder{}` — load_test.go:369,
  file_test.go:252), but package `cmd` cannot touch the unexported var.

**Why an fd-level capture would also be clean here (but is rejected as primary):** the provider
executor wires `cmd.Stdout = &out; cmd.Stderr = &errb` (executor.go:56-57) — the stub subprocess
does **NOT** inherit fd 2. And `runDefault`/cobra write to `cmd.ErrOrStderr()` (the SetErr sink),
not fd 2. So during a full-seam `Execute()`, the **only** writer to the `noticeOut` sink is the §19
notice. Hence a swap (or fd capture) yields exactly the notice text — no noise.

**Chosen capture: add a tiny exported test seam `SetNoticeOut`/`NoticeOut` to `internal/config/file.go`.**
Rationale: deterministic, cross-platform, ~5 lines, and it directly realizes the contract's stated
fact that `noticeOut` is "swappable" (system_context §6 lists it as a test-harness primitive) and the
architecture's explicit plan that "Issue 5 … gets a regression assertion in M1's test subtask (notice
count == 1)" (system_context §5). It does NOT conflict with D1 (D1 scoped the *fix* to skip the second
Load — a test-observability seam is a separate, legitimate concern; production behavior is unchanged:
noticeOut stays `os.Stderr`).

Rejected alternatives (documented in the PRP as fallbacks):
- fd-level `os.Pipe`+`syscall.Dup2` capture: pure zero-source-change, but Unix-only (needs a
  `//go:build !windows` guard) and fiddly (dup/restore/close ordering); no existing `syscall`/build-tag
  usage in the repo to mirror. One-pass risk higher than the seam.
- subprocess (`exec.Command` of the built binary, `cmd.Stderr=&buf`): clean + cross-platform but a NEW
  harness pattern that diverges from the contract-prescribed in-process `rootCmd.SetArgs` seam.

## 3. Exact notice text (for the count assertion)

`file.go:244`: `fmt.Sprintf("stagecoach: repo-local config (.stagecoach.toml) sets provider to %q\n", cfg.Provider)`.
Unique, stable needle: `"repo-local config (.stagecoach.toml) sets provider to"`. Count via
`strings.Count(captured, needle)` → must be **1** (was 2 pre-S1/S2). It fires once per `config.Load`
that runs Layer-3 `loadRepoLocalConfig`. Post-S1/S2 the seam does exactly ONE Load
(`PersistentPreRunE`); `resolveConfig` skips its Load (opts.Config != nil). ⇒ 1.

## 4. Built-ins exclude "stub" (so `[provider.stub]` is genuinely user-defined)

`internal/provider/registry.go:15`: `preferredBuiltins = []string{"pi", "claude", "gemini", "opencode", "codex", "cursor"}`.
`"stub"` is NOT a built-in → a provider named `stub` declared ONLY in a `--config` file is resolved
exclusively via `cfg.Providers` (the user-override map). Before S1/S2 the second Load dropped
`--config` → `reg.Get("stub")` failed → "unknown provider". After: resolved. This is exactly the
Issue-1 regression. (Grep `\"stub\"` in `internal/provider/` → no hits.)

## 5. Why `--config` isolates the provider source for test (a)

`LoadOpts.ConfigPathOverride` ("from --config (CLI); '' => fall back to STAGECOACH_CONFIG, then
discovery" — load.go:14-20): when set, the `--config` file IS the global-file layer (it **replaces**
discovery, not additive). Layer-3 `loadRepoLocalConfig` still runs regardless (only looks at
`.stagecoach.toml`). So test (a) must use a repo with NO `.stagecoach.toml`; isolating HOME/XDG
(loadEnvSetup pattern) is cheap insurance against a stray real global config.

## 6. Pre-validation already honors `--config` (the misleading part of the bug)

`runDefault` validates the provider against `cfg.Providers` (default_action.go ~117-126) BEFORE
calling GenerateCommit — and `cfg` is the first (correct) Load that DID honor `--config`. So the
pre-check always passed; the failure was strictly inside `GenerateCommit`→`resolveConfig`'s second
Load (now skipped). Tests must therefore drive the FULL seam through GenerateCommit (not stop at the
pre-check) to catch the regression — the in-process `rootCmd.SetArgs` harness does exactly this.
