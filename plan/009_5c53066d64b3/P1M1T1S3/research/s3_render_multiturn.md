# Research Note — P1.M1.T1.S3 (RenderMultiTurn sibling method)

## What this subtask does

Add a SIBLING method `RenderMultiTurn` to `internal/provider/render.go` (alongside `Render`). It renders
ONE turn of the multi-turn generation fallback (PRD §9.24 FR-T6) against an existing session id. Render +
its 24+ call sites stay byte-for-byte unchanged (the variadic `mode ...RenderMode` signature carries no
place for a session id — research §2 Option A rejected; Option B sibling is preferred).

S3 touches ONLY `internal/provider/render.go` (+ its test). No other file.

## Dependencies (verified in the working tree)

- **S1 LANDED** — `Manifest.SessionMode *string` (manifest.go:66), Resolve default `strPtr("")`
  (manifest.go:177-178), Validate `""|append` enum (manifest.go:121-123). So `*r.SessionMode` is safe to
  deref after Resolve. S3's capability gate `if *r.SessionMode != "append"` compiles and is correct TODAY.
- **S3 is CODE-INDEPENDENT of S2 and S4.** S3 reads `*r.SessionMode` off the resolved manifest; its UNIT
  tests set `SessionMode: strPtr("append")` directly in a Manifest literal (no registry/merge path). S2
  (MergeManifest clause, in parallel) and S4 (pi builtin `"append"` value) are what make the gate ever PASS
  in production, but S3's logic is correct regardless. S3 does NOT edit merge.go/builtin.go/pi.toml.

## The exact Render pipeline to mirror (render.go:89-159)

```
m.Validate() → r := m.Resolve() → model default fallback →
args = [subcommand]
  + (--provider, inf) if provider_flag!="" && model has "/"  (FR-R5b fold; bare model on pi = HARD ERROR)
  + (model_flag, model) if model_flag!="" && model!=""
  + reasoning tokens if reasoning!="" && ReasoningLevels[reasoning] non-empty (FR-R6, silent no-op)
  + (system_prompt_flag, sys) if system_prompt_flag!="" && sys!=""
  + (mode==tooled ? tooled_flags : bare_flags)   # default bare; tooled errors if empty
  + print_flag if print_flag!=""                  # ALWAYS LAST
payload = userPayload; if system_prompt_flag=="" && sys!="" { payload = sys+"\n\n"+userPayload }  # prepend fallback
switch prompt_delivery: stdin→spec.Stdin | positional→trailing arg | flag→prompt_flag+payload
env = os.Environ() + manifest Env (manifest wins)
```

RenderMultiTurn rebuilds this pipeline with THREE multi-turn deltas (everything else byte-identical):

1. **Capability gate** (after Resolve): `if *r.SessionMode != "append" { return nil, fmt.Errorf("provider %q: multi-turn render requires session_mode=\"append\"", m.Name) }`.
2. **Turn-1-only system prompt** (FR-T6): use a `turnSys` local = `sysPrompt` if `turn==1` else `""`. Substitute `turnSys` for `sysPrompt` in BOTH the flag-emission guard AND the prepend-fallback guard. ⇒ turn>1 emits no `--system-prompt` flag AND does not prepend sys to the payload (the session carries it).
3. **Session-flags block** (FR-T6): replace the bare/tooled mode ternary with a filtered BareFlags block — `BareFlags` MINUS the exact token `"--no-session"`, PLUS `"--session-id", sessionID`. Build a FRESH slice (never mutate `r.BareFlags`).

## Why duplicate the pipeline rather than extract a shared helper

The contract says "rebuilds the §12.2 args EXACTLY as Render does" and "Keeps Render and every existing
caller byte-identical." Extracting a shared internal helper would touch Render's internals — risking the
24+ call sites' byte-identical output (the golden tests `TestRender_Pi_ByteForByteCommitPi`,
`TestRender_GoldenPerProvider` pin Render's exact argv). S3's scope is to ADD a method, not refactor Render.
⇒ Duplicate the args-building in the sibling; both methods have independent golden tests pinning their
output. (A future refactor could DRY this; out of scope for S3.)

## FR-T9 verified flag set (fr-t9-verification.md, 2026-07-05 live run)

pi turn-1 render (stdin delivery):
```
pi --provider zai --model glm-5.2 \
   --system-prompt "<sys>" \
   --no-tools --no-extensions --no-skills --no-prompt-templates --no-context-files \
   --session-id stagecoach-<run-uuid> \
   -p                       < <payload via stdin>
```
- `--no-session` REMOVED from BareFlags; everything else (--no-tools/--no-extensions/--no-skills/--
  --no-prompt-templates/--no-context-files) kept in order.
- `--session-id <id>` APPENDED after the bare-flags block, before `-p`.
- `-p` (print_flag) ALWAYS LAST per §12.2.
- `--continue`/`-c` NOT used (incompatible with `--session-id`).
- Turn 1: `--system-prompt <sys>` present. Turns 2..N+1: `--system-prompt` OMITTED (session carries it).

## The golden turn-1 args (for the byte-for-byte test)

For pi, `model="zai/glm-5.2"`, `sysPrompt="<sys>"`, `sessionID="<id>"`, turn=1, stdin delivery:
```
spec.Command = "pi"
spec.Args    = ["--provider","zai","--model","glm-5.2","--system-prompt","<sys>",
                "--no-tools","--no-extensions","--no-skills","--no-prompt-templates","--no-context-files",
                "--session-id","<id>","-p"]
spec.Stdin   = "<payload>"     # NOT prepended with sys (sys goes via --system-prompt)
```
Turn 2 (same, `turn=2`): identical EXCEPT `--system-prompt`/`<sys>` REMOVED; Stdin still just `<payload>`
(turn>1 ⇒ no prepend). This is the load-bearing turn-1-only assertion.

## Test idiom (render_test.go — established)

- Golden: `wantArgs := []string{...}; if !reflect.DeepEqual(spec.Args, wantArgs) { t.Errorf(...) }`.
- Helpers: `containsToken(args, tok)`, `containsPair(args, flag, val)` (for presence assertions).
- `TestRender_Pi_ByteForByteCommitPi` (render_test.go:90) is the byte-for-byte template — mirror it.
- `TestRender_DoesNotMutateManifest` (render_test.go:271) is the immutability template — the filter must
  build a fresh slice (range over r.BareFlags, never assign into it).

Required tests (4):
1. `TestRenderMultiTurn_PiTurn1_Golden` — the byte-for-byte turn-1 args above (`--no-session` absent;
   `--session-id <id>` present; `--system-prompt <sys>` present; `-p` last; Stdin = payload only).
2. `TestRenderMultiTurn_PiTurn2_NoSysPromptFlag_NoPrepend` — turn 2: no `--system-prompt`; payload NOT
   prepended with sys (Stdin == userPayload exactly).
3. `TestRenderMultiTurn_NonAppendProviderErrors` — capability gate (claude-shape, SessionMode="").
4. `TestRenderMultiTurn_DoesNotMutateManifest` — m.BareFlags still contains "--no-session" after the call.

## Scope (explicitly NOT touched)

- `render.go` `Render` method (byte-identical), `manifest.go` (S1), `merge.go` (S2), `builtin.go` +
  `providers/pi.toml` (S4 — the shipped pi `"append"` value), docs (S5 — render-behavior doc rides with S5),
  `multiturn.go`/generate (P1.M1.T3 — the N+1 protocol that CALLS RenderMultiTurn per turn), any other
  package. S3 is the renderer ONLY; it does not implement the turn loop.

## Validation summary

- `go build ./...` + `go vet ./...` + `gofmt -l .` clean; `go test -race ./...` green.
- The 4 new RenderMultiTurn tests pass; the existing Render golden tests stay byte-identical (unchanged).
- `git diff --stat` → ONLY `internal/provider/render.go` + `internal/provider/render_test.go`.
- Grep: `grep -n "func (m Manifest) RenderMultiTurn" internal/provider/render.go` → 1; the Render method
  itself is unchanged (`git diff internal/provider/render.go` shows only an ADDITION after Render's brace).
