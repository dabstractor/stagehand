# Research — P1.M2.T1.S1: internal/provider/manifest.go — Manifest struct + Render

## Goal of this note
Fix the exact byte-exact expectations for the golden-table tests (oracle = the six
`Rendered:` lines in `external_deps.md` §B.1–B.6) BEFORE writing the PRP, and resolve
the one genuine conflict: the illustrative `Rendered:` line order vs the §12.2
algorithm order.

---

## 1. The three authoritative sources for arg ORDER (they all agree)

| Source | Order statement |
|---|---|
| **PRD §12.2** | `subcommand…, (provider_flag,provider), (model_flag,model), (system_prompt_flag,sys), bare_flags…, print_flag` then delivery switch |
| **decisions.md §5** | "Builds `args []string` in this order: `subcommand…`, (`provider_flag`,`provider`), (`model_flag`,`model`), (`system_prompt_flag`,`sys`), `bare_flags…`, `print_flag`." |
| **Work item LOGIC** | "args = Subcommand; if ProviderFlag!=\"\"&&provider!=\"\" args+=ProviderFlag,provider; … args+=BareFlags; if PrintFlag!=\"\" args+=PrintFlag" |

All three are identical. **print_flag comes AFTER bare_flags.** This is THE contract for this task.

## 2. The §B illustration conflict (RESOLVED — algorithm wins)

The illustrative `Rendered:` lines in §B do NOT all follow the §12.2 order:

- **§B.1 pi** → `pi --provider zai --model … --system-prompt … --no-tools … --no-session -p`
  → `-p` LAST → **matches the algorithm exactly** (it is literally "byte-identical to
  `commit-pi`", the canonical proof the algorithm is correct).
- **§B.2 claude** → `claude -p --model sonnet …` → `-p` FIRST → does NOT match algorithm.
- **§B.6 cursor** → `agent -p --mode ask --trust --model gpt-5 …` → `-p` first, `--model`
  after bare → does NOT match algorithm.

**Resolution:** the work item mandates "**§12.2 byte-exactly**". The algorithm
(§12.2 + decisions.md §5 + work item, all identical) is therefore authoritative for
ORDER. The §B.2/§B.6 lines were written verbatim from the proven shell scripts
(`commit-claude`, the cursor call) which happen to place `-p` first; flag order is
parse-order-independent for those CLIs, so the algorithm-derived order is functionally
identical. The golden tests assert the **algorithm-derived** order; they assert the §B
**flag SET** (which flags/values appear) matches but NOT §B's literal ordering for
claude/cursor. pi is the one case where algorithm == §B literally.

## 3. The system-prompt prepend fallback (applies to stdin AND positional)

`reference_impl.md §3`: "For agents WITHOUT a system-prompt flag
(gemini/opencode/codex/cursor), the system prompt is **prepended** to this
**stdin/positional** payload per PRD §12.2, yielding `<system>\n\n<diff>\n\n<instruction>`."

So the effective payload = `SystemPromptFlag=="" && sys!="" ? sys+"\n\n"+user : user`,
used for stdin payload, positional arg, AND flag arg (all three delivery modes). The
work-item text names the prepend only under the stdin branch; the rendered §B examples
for gemini/opencode/cursor (positional) prove it applies there too. The guard
`&& sys != ""` prevents a stray `"\n\n"` when there is no system prompt.

## 4. The Rendered struct seam (consumed by executor M2.T4.S1)

Work item: `Rendered{Args []string, StdinPayload string, DeliverViaStdin bool, Env []string}`.
executor.go (M2.T4.S1) context confirms the consumption:
- `exec.Command(m.Command, rendered.Args...)` → **`Args` does NOT include the command**;
  the command lives on the Manifest (`m.Command`), which the executor holds alongside.
- `cmd.Stdin = bytes.NewReader(rendered.StdinPayload) if rendered.DeliverViaStdin else nil`
- `cmd.Env = os.Environ() + m.Env` → Rendered.Env is the manifest's env additions only,
  as `[]string` of `"K=V"`. Sorting keys keeps output deterministic (Go map iteration is
  random). The executor merges with `os.Environ()` (so Render stays pure — no os/exec import).

## 5. Render does NOT resolve defaults

The work-item algorithm only guards `model != ""` / `provider != ""`; it never references
`DefaultModel`/`DefaultProvider`. So Render receives **already-resolved** model/provider;
default resolution is the executor/caller's job (M2.T4.S1 / config). This keeps Render a
pure function. `Output`, `JSONField`, `StripCodeFence`, `RetryInstruction` are schema fields
consumed elsewhere (parseOutput M2.T2.S1; retry loop M6); Render does not read them.

## 6. Golden-table computations (sys="SYS", user="BODY" ⇒ prepend="SYS\n\nBODY")

Args shown EXCLUDE the command (command is m.Command).

### pi (§B.1): stdin, has sys-flag, has provider. model="glm-5-turbo" provider="zai"
- effUser="BODY" (sys-flag set ⇒ no prepend)
- wantArgs = `["--provider","zai","--model","glm-5-turbo","--system-prompt","SYS","--no-tools","--no-extensions","--no-skills","--no-prompt-templates","--no-context-files","--no-session","-p"]`
- wantStdin="BODY", wantVia=true

### claude (§B.2): stdin, has sys-flag, NO provider, empty-string bare values. model="sonnet"
- effUser="BODY"; print_flag LAST (algorithm order, NOT §B.2's -p-first)
- wantArgs = `["--model","sonnet","--system-prompt","SYS","--setting-sources","","--tools","","--disable-slash-commands","--no-chrome","--no-session-persistence","-p"]`
- wantStdin="BODY", wantVia=true

### gemini (§B.3): positional, NO sys-flag. model="gemini-2.5-pro"
- effUser="SYS\n\nBODY"
- wantArgs = `["-m","gemini-2.5-pro","--approval-mode","default","SYS\n\nBODY"]`
- wantStdin="", wantVia=false

### opencode (§B.4): positional, subcommand, NO sys-flag. model="anthropic/claude-sonnet-4"
- effUser="SYS\n\nBODY"
- wantArgs = `["run","-m","anthropic/claude-sonnet-4","SYS\n\nBODY"]`
- wantStdin="", wantVia=false

### codex (§B.5): stdin (CORRECTED), subcommand, NO sys-flag. model="gpt-5"
- effUser="SYS\n\nBODY"
- wantArgs = `["exec","-m","gpt-5","--sandbox","read-only","--ask-for-approval","never","--ephemeral"]`
- wantStdin="SYS\n\nBODY", wantVia=true

### cursor (§B.6): positional, print_flag, NO sys-flag. model="gpt-5"
- effUser="SYS\n\nBODY"; model BEFORE bare, print LAST (algorithm order, NOT §B.6's arrangement)
- wantArgs = `["--model","gpt-5","--mode","ask","--trust","-p","SYS\n\nBODY"]`
- wantStdin="", wantVia=false

### Edge cases (also required by the work item)
- **pi no model** (model=""): no `--model` token → `["--provider","zai","--system-prompt","SYS",<bare>,"-p"]`
- **pi no provider** (provider=""): no `--provider` token → `["--model","glm-5-turbo","--system-prompt","SYS",<bare>,"-p"]`
- **flag delivery** (synthetic PromptDelivery="flag", PromptFlag="--prompt", ProviderFlag="--provider", ModelFlag="--model", SystemPromptFlag="--sys", BareFlags=["--x"], PrintFlag="-p"; model="m" provider="p" sys="S" user="U"):
  → effUser="U"; wantArgs=`["--provider","p","--model","m","--sys","S","--x","-p","--prompt","U"]`, wantVia=false, wantStdin=""
- **Env map** (synthetic Env={"B":"2","A":"1"}): Rendered.Env=`["A=1","B=2"]` (sorted by key)
- **unknown delivery** (PromptDelivery="telepathy"): Render returns non-nil error, nil *Rendered
- **stdin no-sys-flag + sys=""** (SystemPromptFlag="" but sys=""): effUser=user (no stray "\n\n")

## 7. TOML struct tags
`external_deps.md §A` sanctions `github.com/pelletier/go-toml/v2`. go-toml/v2 matches TOML
keys to struct fields by reflecting on `toml:"..."` tags (it does NOT auto-convert
CamelCase→snake_case), so `PromptDelivery` would NOT bind to TOML `prompt_delivery` without
an explicit `toml:"prompt_delivery"` tag. The struct is the canonical type config (M5)
imports, so adding `toml:"<snake_key>"` tags now (matching §12.1 keys, incl.
`toml:"env"` on the map) is correct, inert (tags are plain reflect strings — NO go-toml
import needed in manifest.go), and forward-compatible. TOML *parsing/loading* itself is M5;
this task only defines the tagged struct.

## 8. Validation gates (verified valid on host)
go 1.26.4 present; `go test ./...` green (only internal/ui exists). Gates:
`go build ./internal/provider/`, `go vet ./internal/provider/`,
`test -z "$(gofmt -l internal/provider/)"`, `go test ./internal/provider/`, `go test ./...`.
All single-command (no &&/heredoc/for). Scope: ONLY create internal/provider/manifest.go
+ manifest_test.go (new package `provider`); do NOT touch main.go/Makefile/go.mod/go.sum;
do NOT run `go mod tidy` (stdlib-only file).
