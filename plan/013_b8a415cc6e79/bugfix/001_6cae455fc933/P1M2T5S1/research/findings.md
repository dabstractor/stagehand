# Research: P1.M2.T5.S1 — `--verbose` payload-size line for positional/flag-delivery providers (Issue 5)

All claims verified against current source (commit at research time) with exact locations.

## 1. The bug (root cause)

`internal/provider/executor.go` — the ONLY `VerbosePayload` call site:

```go
vb.VerboseCommand(strings.Join(append([]string{spec.Command}, spec.Args...), " "))
vb.VerbosePayload(len(spec.Stdin)) // size only — exposes whether the token-limit gate ran
```

`len(spec.Stdin)` is the delivered payload size ONLY for `stdin`-delivery providers
(pi, claude, agy, qwen, opencode, codex). For `positional` (cursor) and `flag`
(user manifests) delivery, the payload is appended to `spec.Args` and
`spec.Stdin == ""`, so `len(spec.Stdin) == 0`. That 0 hits the guard inside
`VerbosePayload`:

```go
// internal/ui/verbose.go
func (v *Verbose) VerbosePayload(bytes int) {
    if v == nil || v.w == nil || !v.on || bytes <= 0 { return }   // 0 → no-op
    fmt.Fprintf(v.w, "DEBUG: payload: %d bytes (~%d tokens est)\n", bytes, (bytes+3)/4)
}
```

→ NO payload-size line is ever emitted for positional/flag providers. FR50's stated
purpose ("expose whether the token-limit gate actually ran") is defeated for them.

## 2. The data carrier: CmdSpec

`internal/provider/render.go`:

```go
type CmdSpec struct {
    Command string
    Args    []string
    Stdin   string   // payload for stdin delivery; "" → os.DevNull
    Env     []string
}
```

Its doc comment explicitly states: *"CmdSpec intentionally does NOT carry the
delivery mode — Stdin="" disambiguates."* Adding `PayloadBytes` does NOT violate
this — it carries the SIZE (delivery-agnostic), not the mode. But the comment must
be extended to explain why `PayloadBytes` exists alongside the "no delivery mode"
rule, so a future reader doesn't "simplify" it away.

## 3. The TWO delivery switches (both must set PayloadBytes)

### 3a. `Manifest.Render` — switch at render.go ~163–178
```go
spec := &CmdSpec{Command: *r.Command, Args: args}
switch *r.PromptDelivery {
case "stdin":      spec.Stdin = payload
case "positional": spec.Args = append(spec.Args, payload)
case "flag":       spec.Args = append(spec.Args, *r.PromptFlag, payload)
default:           return nil, fmt.Errorf(... unsupported ...)
}
```
The local `payload` (userPayload, optionally prepended with sysPrompt when no
system_prompt_flag) is in scope RIGHT HERE — ideal place to set
`spec.PayloadBytes = len(payload)` for ALL three cases.

### 3b. `Manifest.RenderMultiTurn` — switch at render.go ~272–282
IDENTICAL switch shape; its own `payload` local. Must get the same one-line
addition. (RenderMultiTurn is the multi-turn fallback renderer; PRD §9.24 FR-T6.)

## 4. Delivery-mode roster (builtin.go + manifest.go)
- Constants (manifest.go): `DefaultPromptDelivery = "stdin"`; valid = stdin/positional/flag.
- Builtins: stdin = pi, claude, agy, qwen, opencode, codex; positional = cursor;
  flag = no builtin (user manifests only).
- ⇒ positional/flag are real, supported paths, so the missing verbose line is a
  genuine gap, not dead code.

## 5. The fix (mirrors tasks.json contract §3 + research doc)
1. Add `PayloadBytes int` to `CmdSpec` (render.go:22–29), with a doc comment
   noting it is set by Render for ALL delivery modes so the executor can report
   size without knowing the mode; 0 if no payload.
2. In BOTH Render delivery switches, set `spec.PayloadBytes = len(payload)` for
   all three cases (stdin/positional/flag).
3. In executor.go, change `vb.VerbosePayload(len(spec.Stdin))` →
   `vb.VerbosePayload(spec.PayloadBytes)`.

## 6. CRITICAL non-regression audit — every CmdSpec construction site

Grep `CmdSpec{` across production (.go, non-test) yields exactly THREE sites:

| Site | Constructs via | Stdin | PayloadBytes after fix | Correct? |
|------|----------------|-------|------------------------|----------|
| `render.go:163` Render | Render | per mode | `len(payload)` | YES — set here |
| `render.go:272` RenderMultiTurn | Render | per mode | `len(payload)` | YES — set here |
| `internal/cmd/models.go:151` | DIRECT (not via Render) | `""` | `0` (zero value) | YES — list-models has NO diff payload; no payload line is correct |

**`models.go:151` is the key non-regression:** it bypasses Render and constructs
`CmdSpec{Command: argv[0], Args: argv[1:], Stdin: ""}` for a `--list-models`
command (NOT a diff). Before the fix: `len(spec.Stdin)==0` → no payload line.
After the fix: `spec.PayloadBytes==0` → still no payload line. Behavior unchanged
and correct — a list-models probe has no commit payload to size. ✓ NO fallback
needed in executor; the direct `spec.PayloadBytes` read is safe for all sites.

## 7. Execute callers — all go through Render (except models.go)
Every production `provider.Execute(ctx, *spec, ...)` site dereferences a spec that
came from `Manifest.Render`/`RenderMultiTurn`:
`generate/generate.go:335`, `generate/multiturn.go:165/176/187`,
`generate/workdesc.go:75/106/122`, `decompose/{planner,stager,arbiter,message}.go`,
`hook/exec.go:182`, `pkg/stagecoach/stagecoach.go:569`. All will carry a populated
`PayloadBytes`. Only `models.go` constructs directly (see §6).

## 8. Test patterns to follow (verified extant)

### 8a. Render delivery-mode test — `internal/provider/render_test.go`
`TestRender_FlagDelivery` (L192–204) is the exact template:
```go
m := Manifest{Name: "test", Command: strPtr("agent"),
    PromptDelivery: strPtr("flag"), PromptFlag: strPtr("--prompt")}
spec, _ := m.Render("", "", "PAYLOAD", "off")
// asserts on spec.Args / spec.Stdin
```
Helpers in scope: `strPtr`, `containsPair`, `containsToken`. `RenderMultiTurn`
golden is `TestRenderMultiTurn_PiTurn1_Golden` (render_test.go:499) — uses
`mtPiManifest()` and asserts `spec.Args`/`spec.Stdin`.

### 8b. Executor verbose test — `internal/provider/executor_test.go`
`TestExecute_Verbose` (the verbose keystone) is the template:
```go
var buf bytes.Buffer
vb := ui.NewVerbose(&buf, true)
spec := CmdSpec{Command: "cat", Stdin: "feat: hello\n", Env: os.Environ()}
out, _, err := Execute(context.Background(), spec, 5*time.Second, vb)
got := buf.String()
// assert got contains "DEBUG: command: ...", "DEBUG: raw output: ..."
```
**GOTCHA:** `TestExecute_Verbose` builds CmdSpec DIRECTLY (not via Render) and sets
`Stdin` but NOT `PayloadBytes`. After the fix the executor reads `spec.PayloadBytes`
(==0 here), so the payload line would stop emitting for that test. The test does NOT
currently assert the payload line, so it will NOT fail — but to keep the test
meaningful AND to lock in the new contract, the implementer should either:
  (i) set `PayloadBytes: len("feat: hello\n")` on that spec and ADD an assertion
      that `got` contains `"DEBUG: payload: 12 bytes"`, OR
  (ii) add a NEW dedicated test that constructs a positional-delivery CmdSpec with
      `PayloadBytes` set and asserts the line appears (the regression the fix exists
      for). Recommend doing BOTH: a render-side test (PayloadBytes set per mode) and
      an executor-side test (line emitted from PayloadBytes for positional).

### 8c. No `internal/ui/verbose_test.go` exists for VerbosePayload specifically
`VerbosePayload` is covered transitively through executor tests. No separate ui test
needs to be added for this change (the behavior of VerbosePayload itself is
unchanged — only WHO calls it with what value changes).

## 9. Validation gates (verified Makefile targets)
- Build: `go build ./...`  (or `make build`)
- Tests (race): `go test -race ./...`  (Makefile `test` target)
- Package-focus: `go test ./internal/provider/... -run 'Render|Execute|Verbose' -v`
- Coverage gate (PRD §20.3, ≥85% on internal/provider): `make coverage-gate`
  — `internal/provider` is one of the 4 gated packages, so new tests materially
  help keep/grow coverage here.
- Lint: `make lint` (golangci-lint run; config `.golangci.yml`)

## 10. Scope boundaries (do NOT do)
- Do NOT change `VerbosePayload`'s signature or guard (it stays size-only; §19
  security invariant — never contents).
- Do NOT add a `DeliveryMode` field to CmdSpec (the struct deliberately stays
  mode-agnostic; PayloadBytes is size-only).
- Do NOT add an executor fallback to `len(spec.Stdin)` — the §6 audit proves every
  site is correct with the direct read, and a fallback would re-mask the bug.
- Do NOT change any verbose OUTPUT format (the line stays
  `"DEBUG: payload: %d bytes (~%d tokens est)\n"`); only WHICH providers emit it
  changes (now all three delivery modes).
- No user-facing/config/API/docs surface change (PRD FR50 already documents the
  payload-size line; this makes the implementation match). tasks.json §5: DOCS none.
