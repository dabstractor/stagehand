# P1.M2.T4.S1 — Render research: §12.2 algorithm, golden CmdSpecs, and design calls

> The single source of truth for the renderer. The PRP references this. Read §1 (the algorithm),
> §3 (the golden table), and §4 (the design calls) before writing any code.

## 1. The §12.2 algorithm (AUTHORITATIVE — supersedes the §12.3–12.7 narrative "Rendered" blocks)

From PRD §12.2 (h3.38). This is the precise spec; the per-provider "Rendered:" blocks in §12.3–§12.7
are ILLUSTRATIVE and (for claude §12.4 and cursor §12.7) show a DIFFERENT token order. The algorithm
wins. The existing test `TestBuiltinManifests_RenderedCommand_Cursor` (builtin_test.go:583) ALREADY
documents this exact point:

> "this is the §12.2 ALGORITHM order (renderArgs). §12.7's illustrative "Rendered" block shows
> `agent -p --mode ask --trust --model gpt-5 "<…>"` (different token order). Same tokens; cursor parses
> flags in any order → identical semantics. §12.2 is authoritative (the real P1.M2.T4 renderer)."

Token order produced by the algorithm (memorize this — it is NOT the narrative order):

```
args  = [m.subcommand...]
if m.provider_flag      != "" && provider != "": args += [m.provider_flag, provider]
if m.model_flag         != "" && model    != "": args += [m.model_flag, model]
if m.system_prompt_flag != "" && sys      != "": args += [m.system_prompt_flag, sys]
args += m.bare_flags                       # BARE FLAGS BEFORE PRINT FLAG
if m.print_flag != "": args += [m.print_flag]   # PRINT FLAG IS LAST (confirmed by pi golden test)
# then the prompt_delivery switch (§2 below)
```

**THE headline consequence: `print_flag` is ALWAYS appended LAST (after bare_flags)**, for EVERY
provider — including claude (§12.4 narrative wrongly shows `-p` first) and cursor. The pi
byte-for-byte test (`TestBuiltinManifests_RenderedCommand_Pi_MatchesCommitPi`, builtin_test.go:339)
pins `-p` last with the comment "§12.2: print_flag LAST (matches §12.3 + commit-pi)". §12.3's pi
"Rendered" block agrees (algo + narrative coincide for pi); only claude/cursor narratives diverge.

## 2. The prompt_delivery switch + the stdin / system-prompt prepend fallback

PRD §12.2 pseudocode + the "Note on system prompt + stdin". Unified into ONE `payload` string that is
reused for stdin content, the positional arg, and the flag arg:

```
payload = userPayload
sysFlag := resolvedManifest.SystemPromptFlag      # "" when the agent has no sys-prompt flag
if sysFlag == "" && sys != "":
    payload = sys + "\n\n" + userPayload            # PREPEND fallback (gemini/opencode/codex/cursor)
# else: payload stays userPayload only (sys already went via the flag, e.g. pi/claude)

switch resolvedManifest.PromptDelivery:
case "stdin":      spec.Stdin = payload                     # NOTHING appended to args
case "positional": args = append(args, payload)
case "flag":       args = append(args, promptFlag, payload)
```

Why the unified `payload` is correct for ALL SIX built-ins (verified against §12.3–§12.7):

| provider  | delivery   | sys_flag       | sys handling              | payload →            |
|-----------|-----------|----------------|---------------------------|----------------------|
| pi        | stdin     | --system-prompt| via FLAG                  | userPayload only     |
| claude    | stdin     | --system-prompt| via FLAG                  | userPayload only     |
| gemini    | stdin     | "" (none)      | PREPEND (no flag)         | sys + "\n\n" + user  |
| opencode  | positional| "" (none)      | PREPEND                   | sys + "\n\n" + user  |
| codex     | stdin     | "" (none)      | PREPEND                   | sys + "\n\n" + user  |
| cursor    | positional| "" (none)      | PREPEND                   | sys + "\n\n" + user  |

The delimiter between sys and user is exactly `"\n\n"` (every §12.5–§12.7 narrative writes
`"<sys>\n\n<user payload>"`). When `sys == ""`, do NOT prepend (payload = userPayload; no leading
newlines). This is robust to the new-repo path where the system prompt may be shorter but is never
empty in practice — and the empty-sys guard prevents malformed payloads either way.

## 3. The golden CmdSpec table (assert these EXACTLY in the table-driven tests)

Inputs unless noted: `sys="<sys>"`, `user="<user>"`. Command shown separately from Args (CmdSpec
shape). `model`/`provider` params shown per row. Args = flags portion AFTER the prompt_delivery
switch (positional/flag append the payload; stdin does NOT).

```
# pi — provider="zai", model="" (→ default glm-5-turbo). THE byte-for-byte commit-pi check.
Command = "pi"
Args    = ["--provider","zai",
           "--model","glm-5-turbo",
           "--system-prompt","<sys>",
           "--no-tools","--no-extensions","--no-skills","--no-prompt-templates","--no-context-files","--no-session",
           "-p"]
Stdin   = "<user>"          # stdin delivery; sys_flag exists → sys via flag, only user via stdin
Env     = os.Environ()      # + pi.Env (nil → no extra entries)

# claude — provider="", model="sonnet". (model passed explicitly; default is "sonnet" anyway)
Command = "claude"
Args    = ["--model","sonnet",
           "--system-prompt","<sys>",
           "--tools","","--setting-sources","","--no-session-persistence",
           "-p"]            # print_flag LAST (NOT first like the §12.4 narrative!)
Stdin   = "<user>"

# gemini — provider="", model="" (→ default gemini-2.5-pro)
Command = "gemini"
Args    = ["-m","gemini-2.5-pro",
           "--approval-mode","default"]
Stdin   = "<sys>\n\n<user>"  # stdin delivery; no sys_flag → sys PREPENDED

# opencode — provider="", model="anthropic/claude-sonnet-4" (explicit; default is "")
Command = "opencode"
Args    = ["run",
           "-m","anthropic/claude-sonnet-4",
           "<sys>\n\n<user>"]  # positional delivery → payload IS the trailing arg
Stdin   = ""                  # positional → no stdin pipe

# codex — provider="", model="gpt-5" (explicit; default is "")
Command = "codex"
Args    = ["exec",
           "-m","gpt-5",
           "--sandbox","read-only","--ephemeral"]
Stdin   = "<sys>\n\n<user>"   # stdin delivery (REVISED builtin); no sys_flag → sys PREPENDED

# cursor — provider="", model="gpt-5" (explicit; default is "")
Command = "agent"             # cursor is the ONLY provider where Command != Name
Args    = ["--model","gpt-5",
           "--mode","ask","--trust",
           "-p",              # print_flag LAST per §12.2 (NOT first like the §12.7 narrative!)
           "<sys>\n\n<user>"] # positional delivery → payload IS the trailing arg
Stdin   = ""
```

These Args are BYTE-COMPATIBLE with the existing `renderArgs(...)` outputs in builtin_test.go
(renderArgs returns Command as element[0]; CmdSpec splits Command out — same tokens, same order).
pi specifically is byte-for-byte the commit-pi invocation.

## 4. Design calls (the decisions most likely to be implemented wrong)

### Call A — placement: NEW file `internal/provider/render.go`; do NOT edit `manifest.go`.
The parallel PRP P1.M2.T3.S1 (registry) FROZE `manifest.go` (it must not edit S1's contract to preserve
the struct/Validate/Resolve/DetectCommand). Adding code to `manifest.go` now risks a merge collision
with the parallel registry work. Go allows a type's methods to live in ANY file of the same package, so
placing `(m Manifest) Render(...)` in `render.go` STILL makes it a "method of the Manifest type"
(satisfying the work item's "manifest method" phrasing) without touching the frozen file. `render.go` is
the ONLY new non-test file. `CmdSpec` is declared in the same `render.go` (tightly coupled to Render).

### Call B — signature: METHOD `func (m Manifest) Render(model, provider, sysPrompt, userPayload string) (*CmdSpec, error)`.
The work-item contract writes `Render(m Manifest, ...)` (m as first PARAM). The METHOD form
(`(m Manifest) Render(model, ...)`) is idiomatic Go (the manifest renders itself), avoids passing m
twice, and reads better at the call site (`m.Render(...)` vs `provider.Render(m, ...)`). 4 params
(model/provider/sys/user) — the work-item's `m` becomes the receiver. Justification noted; either form
is acceptable but method is preferred. Downstream callers (executor T5 / generate M3.T4) call `m.Render(...)`.

### Call C — Render calls `Validate()` THEN `Resolve()` internally (defensive; returns error).
The registry PRP's documented consumer lifecycle is `Get → Validate → Resolve → consume`. Render OWNS
the "Validate → Resolve → consume" tail so it is robust to a caller that skipped those steps: it calls
`m.Validate()` (returns its error → covers "Validate prompt_delivery mode" from the work item + missing
Command/Name), then `m.Resolve()` (every pointer becomes non-nil → safe deref, no nil-panics). Resolve
returns a COPY (it never mutates the caller's m). The switch's default case (unrecognized
prompt_delivery) returns a wrapped error too — belt-and-suspenders since Validate already rejects it.

### Call D — model/provider DEFAULTS applied inside Render (when the param is "").
Mirrors the existing `renderArgs` test scaffolding (builtin_test.go:137): `modelToUse = model ||
*r.DefaultModel`. This makes the pi golden test pass with `model=""` (→ glm-5-turbo) and is convenient
for callers (they MAY pass "" to mean "use the manifest default", or pass an explicit value to override).
Symmetrically `providerToUse = provider || *r.DefaultProvider` — harmless for every built-in (after
Resolve their DefaultProvider is "" → no --provider emitted unless the caller passes one). A §12.8 user
manifest with `default_provider = "zai"` is correctly honored when the caller passes provider="".

### Call E — CmdSpec.Stdin semantics: payload for stdin delivery; "" for positional/flag.
CmdSpec is `{Command, Args, Stdin, Env}` per the work item — it does NOT carry the delivery mode, so
Stdin="" disambiguates "no stdin pipe". The executor (P1.M2.T5) contract becomes: `if spec.Stdin != ""
{ cmd.Stdin = strings.NewReader(spec.Stdin) } else { cmd.Stdin = <os.DevNull> }`. This matches PRD §12.2's
`cmd.Stdin = (delivery=="stdin") ? reader : /dev/null`. The degenerate edge (stdin delivery with an empty
payload → Stdin="" → executor uses /dev/null) is functionally harmless (EOF to the child either way) and
never occurs in the real flow (the payload always contains the diff/instruction).

### Call F — Env = append(os.Environ(), "KEY=VAL" ...); manifest env WINS on collision (last wins).
PRD §12.2: `cmd.Env = os.Environ() + m.env`. Convert each `m.Env` entry to `"KEY=VAL"` and append AFTER
`os.Environ()` so `exec.Cmd.Env`'s last-wins semantics make manifest env override the parent's.
TESTABILITY: os.Environ() is machine-dependent → tests MUST NOT assert full Env equality. Assert (a)
each manifest env entry appears as `"KEY=VAL"` in spec.Env, and (b) `len(spec.Env) >= len(os.Environ())`.

### Call G — the sys-prepend check uses the RESOLVED SystemPromptFlag, not the raw pointer.
After Resolve, SystemPromptFlag is non-nil ("" when absent). The prepend condition is
`*resolved.SystemPromptFlag == "" && sys != ""`. Do NOT test the raw `m.SystemPromptFlag` pointer (a
provider with an absent flag has it nil pre-Resolve — Resolve normalizes it to *"" so the check is uniform).

## 5. Byte-compatibility note: Render vs the existing test-only `renderArgs`

`renderArgs` (builtin_test.go:137) is TEST-ONLY scaffolding that builds the FLAGS-ONLY argv (it does
NOT assemble the payload, stdin, or Env, and it returns Command as element[0]). Render is the REAL
renderer producing a full CmdSpec. Render's Args (flags portion) are IDENTICAL to renderArgs's output
(renderArgs = Command + flags; CmdSpec = Command separate + Args = flags). So every existing
`TestBuiltinManifests_RenderedCommand_*` golden argv is directly reusable as Render's Args expectation
(minus splitting Command out). The renderArgs helper is NOT deleted (it's in a frozen test file) and NOT
reused by Render (Render implements §12.2 standalone) — they coexist.

## 6. Imports for render.go

`os` (os.Environ), `fmt` (error wrapping), `strings` (NOT strictly needed — `+` concatenation suffices
for the payload; `strings.NewReader` is the EXECUTOR's job, not Render's). If no `strings` use, omit it
(go vet flags unused imports). Final import set: `fmt`, `os`. (No go-toml, no os/exec — Render does not
spawn anything; that's the executor T5.)
