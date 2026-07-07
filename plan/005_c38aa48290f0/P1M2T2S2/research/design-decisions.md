# P1.M2.T2.S2 — design decisions & load-bearing findings

Research date: 2026-07-02. Source: live codebase + PRD snapshot §9.19 FR-F8, §9.7, §17.8, §9.22 FR-E1/E3.

## 1. The three commit paths and WHERE the seam goes (verified in source)

The message a run commits is "accepted" at the SAME structural point in three parallel generate/dedupe
loops, plus the FR-M11 planner shortcut. `--template` must be applied **after `provider.ParseOutput` succeeds
and BEFORE `ExtractSubject`/`IsDuplicate`** so §9.7 judges the final (templated) subject (FR-F8 explicit).

| Path | File | Accept point (insert seam here) |
|------|------|--------------------------------|
| single-commit | `internal/generate/generate.go` | after `parseFail = false` (L237), before `signal.SetCandidate(m)` (L238) |
| public API dry-run/system-extra | `pkg/stagecoach/stagecoach.go` `runPipeline` | after `parseFail = false` (L518), before `signal.SetCandidate(m)` (L519) |
| decompose message role | `internal/decompose/message.go` `generateMessage` | after `parseFail = false` (L161), before `subject := generate.ExtractSubject(m)` (L163) |
| FR-M11 single shortcut | `internal/decompose/decompose.go` `runSingleShortcut` | template `plannerMsg` (L322) BEFORE the `dupCheckMessage` (L323) call |

The rescue candidate (`signal.SetCandidate`, `candidate = m`, `rejected = append(...)`, `msg = m`) must all
observe the TEMPLATED message ⇒ apply the seam BEFORE `SetCandidate`, at the single point above.

## 2. Arbiter N+1 / one-file / escape hatch are covered TRANSITIVELY — no extra edits

- **Arbiter null → (N+1) commit** (`chain.go` `resolveNewCommit`, L109): message comes from
  `generateMessage(...)` → runs the decompose loop → **already templated**. Nothing to add.
- **Arbiter tip-amend / mid-chain** (`chain.go` `resolveTipAmend`/`resolveMidChain`): REUSE an existing
  in-run commit message (already templated when that commit was made). Nothing to add.
- **One-file short-circuit** (FR-M2b, `decompose.go` L179): generates via `generateMessage` → templated.
- **Escape hatch `--single`/`--commits 1`** (`runSingleEscape`): delegates to `generate.CommitStaged` →
  templated by the generate.go seam.

Net: **4 explicit insertion points** (3 loops + `runSingleShortcut`). Everything else inherits the seam.
This is the "applied to EVERY commit message in a run" guarantee (FR-F8), achieved with minimal surface.

## 3. Seam design: `FinalizeMessage` + `ApplyTemplate` in `internal/generate`

`internal/generate` is imported by BOTH `decompose` and `pkg/stagecoach` (they already call
`generate.ExtractSubject` / `generate.IsDuplicate` / `generate.RescueError`). It also imports `config`.
So it is the single shared home for the seam — no new import edges, no import cycle.

```go
// internal/generate/finalize.go  (NEW)
func ApplyTemplate(msg, tpl string) string { if tpl == "" { return msg }; return strings.ReplaceAll(tpl, "$msg", msg) }
func FinalizeMessage(msg string, cfg config.Config) string { return ApplyTemplate(msg, cfg.Template) }
```

`FinalizeMessage` is the NAMED seam the item asks for — "a single ordered message-finalization pipeline".
Today it is one stage (template). **Ordering contract for P1.M5.T1.S1**: the `--edit` editor gate slots
AFTER this seam (FR-E3: template applied before the editor opens). Physically the editor runs after the
dedupe accept / before `commit-tree`; the seam here is the template stage that precedes it. The docstring
states the contract so S1's author inserts the editor after template, not before.

`$msg` semantics: literal substitution of the FULL message (subject+body), `strings.ReplaceAll` (all
occurrences). `$msg`-only template ⇒ identity. Empty template ⇒ identity (byte-identical to today — the
default). Multi-line message + `"$msg (#205)"` ⇒ suffix lands after the body (spec: "$msg = full message");
dedupe then extracts the unchanged first line. This is the literal, spec-faithful reading of FR-F8 — do NOT
try to be clever and inject into the subject only (that would violate "the full generated message").

## 4. Config plumbing: mirror `format`/`locale` EXACTLY (full 5-layer precedence)

Unlike `--context` (S2.T2.S1, flag-only), `--template` has the FULL surface:
`[generation].template` / `stagecoach.template` / `STAGECOACH_TEMPLATE` / `--template`. The `format`/`locale`
scalars are the byte-for-byte precedent — add `Template` in the same six spots:

1. `config.go`: field `Template string \`toml:"template"\`` (after `Locale` L89); `Defaults()` `Template: ""` (after L154).
2. `file.go`: `fileGeneration.Template` (after L60); materialize `if g.Template != "" { c.Template = g.Template }` (~L243); merge `if src.Template != "" { dst.Template = src.Template }` (~L374).
3. `git.go`: `stagecoach.template` `gitConfigGet` block (after L139).
4. `load.go` `loadEnv`: `STAGECOACH_TEMPLATE` (after L247).
5. `load.go` `loadFlags`: `--template` via `fs.Changed` (after L345).
6. `root.go`: `flagTemplate` var + `pf.StringVar` registration (after the `--locale` registration).

Validation: `validateTemplate(cfg.Template)` at the Load() tail, immediately AFTER `validateFormat` (L169),
mirroring it exactly (PURE, called ONCE on the fully-resolved value). Rule: `tpl == "" || strings.Contains(tpl, "$msg")`.

## 5. `config init --template` collision — SAFE, but must be tested

`config init` registers a LOCAL bool flag `--template` (`internal/cmd/config.go` L144). The new global
`--template` is a root PERSISTENT string flag. Cobra's `mergePersistentFlags` calls `Flags().AddFlagSet(parentsPflags)`,
and pflag's `AddFlagSet` SKIPS any name already present locally — so for `config init` the LOCAL bool wins;
the inherited string is not added. No panic, no type clash. `stagecoach --template '$msg (#205)'` uses the
string; `stagecoach config init --template` uses the bool. Disambiguate via help text (item note) and add a
regression test that both parse (`config init --template` stays a bool; root `--template x` sets `cfg.Template`).

## 6. Interaction with format modes / multi-line / dedupe rejection list

- Template is orthogonal to `--format` (S3): `gitmoji` "🐛 fix" + `"$msg (#205)"` ⇒ "🐛 fix (#205)". No special-casing.
- On a duplicate, the appended rejected subject is the TEMPLATED subject (what would land). Correct per
  FR-F8 ("judge the final subject"); the agent re-generates a bare message and the template re-applies.
- Empty-template path MUST stay byte-identical to today — the seam short-circuits on `tpl == ""`. This is the
  load-bearing regression guard (all existing generate/decompose/stagecoach tests pass unchanged).
