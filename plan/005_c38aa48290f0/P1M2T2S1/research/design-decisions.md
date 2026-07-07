# P1.M2.T2.S1 — `--context` user-payload injection: design decisions & codebase evidence

## 1. The exact ordering contract (§17.8 + work item)

Work item OUTPUT clause: *"instruction → context → rejection → diff"*.

§17.8 (Context, FR-F7): the context block is inserted into the **user payload** (message + planner
roles) *"after the instruction line and before the diff — the same slot the duplicate-rejection block
occupies (§17.3), and **before it when both are present**."*

Resulting assembly for `BuildUserPayload` (message role):

| Case | Assembly |
|---|---|
| no context, no rejection | `userInstruction(":") + "\n\n" + diff` (**byte-identical to today**) |
| context, no rejection | `userInstruction(":") + "\n\n" + CONTEXTBLOCK + "\n\n" + diff` |
| no context, rejection | `userInstructionReject(".") + "\n\n" + <rejection block> + diff` (**byte-identical to today**) |
| context + rejection | `userInstructionReject(".") + "\n\n" + CONTEXTBLOCK + "\n\n" + <rejection block> + diff` |

Where `CONTEXTBLOCK` = `"Additional context from the user (treat as authoritative):\n" + text`
(exactly one `\n` between the header line and the text, per the §17.8 literal block; NO trailing newline —
the assembler owns the `"\n\n"` that follows, matching payload.go's existing constant discipline).

## 2. Colon-vs-period is NOT changed by context (load-bearing call)

§17.3's instruction punctuation is governed **solely by `len(rejected)`** (colon = normal introduces the
diff; period = rejection, the IMPORTANT directive follows). §17.8 says only to *insert* the context block
into the existing slot; it does **not** instruct changing the instruction line. Therefore context injection
must **not** alter which instruction constant is used. This keeps the `context==""` path byte-identical to
today (the load-bearing regression guard — mirrors S3's `auto`+empty-locale byte-identity guard) and is the
minimal, most spec-literal reading. Documented in the PRP gotchas; pinned by canonical-exact tests.

## 3. Flag-only resolution (FR-F7) — NO env / config / git key

Work item CONTRACT §1: *"Flag-only by spec (FR-F7: per-invocation by nature) — no env, no config key, no
git key."* This deliberately breaks the standard 5-layer precedence that `--format`/`--locale` follow. The
codebase precedent for a flag-only Config field with no file source is **`flagExclude`** (root.go:69, "never
read directly … via fs.Changed/fs.GetStringArray in config.Load's loadFlags") and the `Exclude` field which
has *no env var, no git-config key* (config.go:95-99). Context is even simpler (a scalar, replace-semantics).

`Config` itself is **never decoded from a TOML file** — a separate `fileConfig` carries file values
(config.go:105-107 comment: *"Config is never decoded from §16.2; fileConfig is"*). So a `Context` field on
`Config` can never be populated from a file regardless. We add `toml:"-"` anyway for intent-clarity and to be
defensive. No `[generation].context` key is added to `fileConfig`, no `STAGECOACH_CONTEXT` env read in
`loadEnv`, no `stagecoach.context` git read — only a `fs.Changed("context")` block in `loadFlags`.

## 4. Planner injection (FR-F7: message AND planner roles)

`BuildPlannerUserPayload(diff, forcedCount)` (planner.go:109) gets the same slot: context after the
`plannerUserInstruction` line, before the diff. Forced-count mode prepends `"Produce EXACTLY N…"` **before**
`plannerUserInstruction`; context still goes **after** `plannerUserInstruction`, before the diff:

- normal: `plannerUserInstruction + "\n\n" + CONTEXTBLOCK + "\n\n" + diff`
- forced: `forced + "\n" + plannerUserInstruction + "\n\n" + CONTEXTBLOCK + "\n\n" + diff`

There is no rejection block in the planner payload, so context is the only optional block.

## 5. Roles explicitly UNCHANGED (scope fence)

- **Stager** (`internal/prompt/stager.go`, `BuildStagerTaskPayload`) — NO context (work item CONTRACT §3).
- **Arbiter** (`internal/prompt/arbiter.go`) — NO context.
- Hook `exec` takes **no** `--context` (git invokes the hook; no flag surface at commit time) — N/A per
  work item; nothing to build here, just noted for the hook subtask (P1.M3.T2.S1).

## 6. Call sites (4 message + 1 planner, all confirmed by grep)

`grep -rn 'BuildUserPayload\|BuildPlannerUserPayload' --include='*.go' | grep -v '_test.go'`:
- `internal/generate/generate.go:194` — `prompt.BuildUserPayload(diff, rejected)` (cfg = `cfg`)
- `internal/decompose/message.go:119` — `prompt.BuildUserPayload(diff, rejected)` (cfg = `deps.Config`)
- `pkg/stagecoach/stagecoach.go:481` — `prompt.BuildUserPayload(diff, rejected)` (cfg = `cfg`)
- `internal/decompose/planner.go:85` — `prompt.BuildPlannerUserPayload(diff, forcedCount)` (cfg = `deps.Config`)

The message builder is called INSIDE the retry loop each attempt (rejection list grows across attempts);
`cfg.Context` is loop-invariant so it is passed identically every attempt — correct.

## 7. Verified S3 contract dependency (parallel implementation)

S3 (P1.M2.T1.S3, implementing in parallel) touches the **system-prompt** builders
(`BuildSystemPrompt`/`BuildFallbackPrompt`/`BuildPlannerSystemPrompt`) and does NOT touch `payload.go` or
`BuildUserPayload`/`BuildPlannerUserPayload` (S3 PRP line 12 + "OUT OF SCOPE" line 567: *"payload.go /
BuildUserPayload (--context is S2.T2.S1)"*). **Zero file overlap** — S3 edits system.go/planner.go's
system-prompt funcs; this subtask edits payload.go and planner.go's *user-payload* func
(`BuildPlannerUserPayload`) which S3 leaves untouched. The one shared file is `planner.go`, but disjoint
functions (`BuildPlannerSystemPrompt` = S3; `BuildPlannerUserPayload` = here).
```

## 8. docs/cli.md row format (Mode A doc)

6-column table: `| Flag | Type | Default | Env | Git config | Description |`. Insert after the `--locale`
row (line 39). Env + Git-config columns are `—` (flag-only, FR-F7).
