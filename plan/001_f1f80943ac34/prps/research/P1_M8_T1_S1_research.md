# Research — P1.M8.T1.S1: providers/{pi,claude,gemini,opencode,codex,cursor}.toml

## Task
Ship six contributor-facing reference `providers/<name>.toml` files that are
**golden-equivalent** to the six manifests compiled into the binary by
`internal/provider/builtin.go` (P1.M2.T3.S1). Add a golden-equivalence test so
the on-disk copies and the compiled built-ins can never drift.

## Authoritative input (DO NOT re-derive — copy field values from here)
`internal/provider/builtin.go` `Builtins()` is the single source of truth. Its
values already encode the four `external_deps.md §C` corrections:
- §C.1 claude: 5 logical bare flags encoded as a **7-element** slice
  (`--setting-sources`,``,`--tools`,``,`--disable-slash-commands`,`--no-chrome`,`--no-session-persistence`).
- §C.2 codex: `prompt_delivery = "stdin"` (NOT positional) + `--ephemeral`.
- §C.3 gemini: positional delivery, empty `print_flag`.
- §C.4 cursor: command/detect = `"agent"`, `--mode ask --trust`.

`internal/provider/builtin_test.go` (`TestBuiltins_MatchesOracle`) restates the
same six manifests as an independent oracle — use it as the per-field checklist.

## Exact `toml.Marshal(Builtins()[name])` output (verified, go-toml/v2 v2.4.2)
go-toml/v2 marshals EVERY field (incl. zero values), uses single quotes, inline
arrays, struct-definition order. Example (pi):
```
name = 'pi'
detect = 'pi'
command = 'pi'
subcommand = []
prompt_delivery = 'stdin'
prompt_flag = ''
print_flag = '-p'
model_flag = '--model'
default_model = 'glm-5-turbo'
system_prompt_flag = '--system-prompt'
provider_flag = '--provider'
default_provider = ''
bare_flags = ['--no-tools', '--no-extensions', ...]
output = 'raw'
json_field = ''
strip_code_fence = true
retry_instruction = 'Output ONLY the commit message. No preamble, no markdown, no quotes.'
```
**The shipped files are NOT raw marshal output** (task requires a human-readable
header comment + multi-line arrays). "Byte-consistent" therefore means
**decodes to the same `Manifest`** (golden equivalence), NOT byte-identical text.

## go-toml/v2 decode gotchas (drive the golden test's normalization)
1. Absent key → Go zero value: nil slice/map, `""` string, `false` bool.
2. `key = []` → **non-nil** `[]string{}`; absent key → **nil** slice. These are
   NOT `reflect.DeepEqual`, so the golden test MUST normalize nil/empty slices.
3. `[env]` table present-but-empty → **non-nil** `map[string]string{}`; absent →
   nil. ⇒ The .toml files should OMIT the `[env]` table (all six builtins have
   nil Env) so decode → nil, matching. The test should also normalize Env maps
   defensively.
4. Inline `#` comments and multi-line arrays are valid TOML and ignored on
   decode → safe to use for Mode A inline docs and readable arrays.

⇒ **Reuse the existing `manifestsEqual`/`normalizeEmpty` helpers from
`builtin_test.go`** (same `package provider` test binary) and EXTEND with Env
map normalization (or write a local `goldenManifestsEqual`). The prototype
confirmed a commented, multi-line, double-quoted `pi.toml` decodes equal to
`Builtins()["pi"]`.

## Per-file required NON-ZERO fields (zero-value fields may be omitted for
readability — both decode to the builtin's zero value; see gotcha #1)
- **pi**: name,detect,command=pi; prompt_delivery=stdin; print_flag=-p;
  model_flag=--model; default_model=glm-5-turbo; system_prompt_flag=--system-prompt;
  provider_flag=--provider; bare_flags(6); output=raw; strip_code_fence=true;
  retry_instruction="Output ONLY...". ← **ONLY provider with non-empty retry_instruction.**
- **claude**: prompt_delivery=stdin; print_flag=-p; model_flag=--model;
  default_model=sonnet; system_prompt_flag=--system-prompt; bare_flags(7); output=raw; strip_code_fence=true.
- **gemini**: prompt_delivery=positional; model_flag=-m; default_model=gemini-2.5-pro;
  bare_flags=[--approval-mode,default]; output=raw; strip_code_fence=true. (no print_flag, no system_prompt_flag)
- **opencode**: subcommand=[run]; prompt_delivery=positional; model_flag=-m;
  output=raw; strip_code_fence=true. (bare_flags absent/empty)
- **codex**: subcommand=[exec]; prompt_delivery=stdin; model_flag=-m;
  bare_flags=[--sandbox,read-only,--ask-for-approval,never,--ephemeral]; output=raw; strip_code_fence=true.
- **cursor**: name=cursor; detect=agent; command=agent; prompt_delivery=positional;
  print_flag=-p; model_flag=--model; bare_flags=[--mode,ask,--trust]; output=raw; strip_code_fence=true.

All others (prompt_flag, default_model where "", default_provider, json_field,
retry_instruction except pi, system_prompt_flag except pi/claude, subcommand
except opencode/codex, provider_flag except pi) are zero → OMIT in the file.

## Header comment / TO-CONFIRM notes per file (PRD §12.7.2, external_deps §B)
- pi: verified; byte-identical to `commit-pi`; no TO-CONFIRM.
- claude: §C.1 — note the 5-flag bare set; `--system-prompt` REPLACES default
  persona (use `--append-system-prompt` to add).
- gemini: TO-CONFIRM delivery (E.1) — positional default; stdin
  preferred-if-verified at integration; `-p` is DEPRECATED (do not use).
- opencode: user must set model as `provider/model` (default_model="").
- codex: TO-CONFIRM (E.4a / §12.7.2) — stdin delivery + `--ephemeral`.
- cursor: TO-CONFIRM (E.4b / §12.7.2) — `--mode ask`=read-only strongly
  indicated; binary is `agent` (some installs: `cursor agent`).

## Test placement (INTERNAL IMPORT RULE — verified)
`internal/provider` is an internal package: a test outside the module CANNOT
import it (prototype `goldenprov` failed with "use of internal package ... not
allowed"). ⇒ The golden test MUST live in-package:
`internal/provider/providers_golden_test.go` (`package provider`), reading
`filepath.Join("..","..","providers", name+".toml")`. `go test` sets CWD to the
package dir, so `../../providers/` resolves to repo-root `providers/`. Verified
to PASS for a real `pi.toml`.

Recommend the test **FAIL** (not skip) if a file is missing or decodes unequal —
that is the entire point of "never drift".

## Validation gates (each ONE simple command, verified in this repo)
1. `go build ./...`
2. `go test ./internal/provider/ -run TestProvidersTOML_MatchBuiltins -v`
3. `go test ./...`
4. `go vet ./internal/provider/`

## DOCS impact (Mode A)
Each `.toml` file IS its own Mode A doc (header comment explains the provider;
inline comments explain each field). No separate `docs/` change for this item;
changeset-level README/docs consolidation is a later M8 task (Mode B).

## Scope boundaries (do NOT do here)
- Do NOT modify `internal/provider/builtin.go` (authoritative source — M2.T3.S1).
- Do NOT wire the .toml files into the runtime config loader (they are
  REFERENCE copies; built-ins are compiled in). User overrides still come via
  config-file `[provider.<name>]` tables (M5.T3.S1).
- Do NOT add an `[env]` table to any file (all six builtins have nil Env).
