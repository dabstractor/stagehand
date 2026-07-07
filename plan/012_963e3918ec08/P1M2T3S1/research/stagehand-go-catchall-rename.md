# Research: Catch-all `stagehand`→`stagecoach` rename in `.go` (error prefixes + the real 377-site residue)

> Subtask P1.M2.T3.S1. The contract TITLE says "error message prefixes and status/progress strings"
> (~20 sites), but the contract MECHANISM is a broad `sed s/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g`
> and the OUTPUT gate is "Zero occurrences of 'stagehand' (case-insensitive) in any .go file." The real
> residue is **377 occurrences across ~60 .go files** — the title vastly underestimates the catch-all scope.
> This PRP follows the MECHANISM + OUTPUT (the catch-all), not the title's narrow count.

---

## 1. THE critical finding — the real scope is 377, not ~20

`grep -rni 'stagehand' --include='*.go' . | grep -v '.git/' | grep -v '.pi-subagents' | wc -l` → **377**.
The contract's "~20+ error-prefix locations" (migrate.go/load.go/file.go/generate.go/…, Layer 3.2) is a
SERIOUS underestimate. The 377 span nearly every package:

```
17 internal/cmd    6 internal/generate    6 internal/config    5 internal/hook
 4 internal/provider   4 internal/prompt   4 internal/decompose   3 internal/integrate
 3 internal/hooks   3 internal/e2e   2 pkg/stagecoach   2 internal/ui   2 internal/signal
 1 internal/stubtest  1 internal/lock  1 internal/exitcode  1 cmd/stagecoach
```

The contract's broad-sed MECHANISM is precisely the right tool for this: the residue is scattered comments +
straggler strings, not a tidy 20-line list. **Follow the mechanism (broad sed) + the zero-residue OUTPUT
gate, not the title's count.** (rename_surface_map.md Layer 3.2-3.11 maps the categories; M1.T2 already
renamed the IDENTIFIERS + most VALUES — what remains is comments + a few straggler strings M1.T2 missed.)

## 2. WHY the broad sed is SAFE (the contract's CAUTION, resolved)

The contract: "this broad sed must NOT run before verifying it won't break already-renamed identifiers."
**Verified:** all 377 hits are **comments + string literals** — ZERO identifiers, ZERO import paths. M1.T1
(imports/module) + M1.T2 (identifiers) + M2.T1 (env vars `STAGECOACH_*`) + M2.T1 (git-config keys
`stagecoach.*`) already landed. The remaining `stagehand`/`Stagehand` tokens live ONLY in:
- `// ...` doc comments (the bulk — pure prose, zero semantic value)
- `"stagehand: ..."` error-prefix string literals (Layer 3.2 — the title's scope)
- a handful of straggler user-facing strings (Layer 3.3/3.6/3.8/3.10/3.11) + their test assertions

A blind `s/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g` therefore CANNOT corrupt an identifier or
import (there are none left to corrupt). `go build ./...` is the gate that proves it.

**No all-caps `STAGEHAND` residue** (`grep -rn 'STAGEHAND' --include='*.go'` → empty — M2.T1.S1 renamed the
env vars). So the two-variant sed (`stagehand`/`Stagehand`) covers everything. (If a `STAGEHAND` straggler
appears later, the zero-residue `-rni` gate catches it → add `s/STAGEHAND/STAGECOACH/g`.)

## 3. The semantic strings — production + test rename IN LOCKSTEP (sed keeps them coordinated)

A handful of straggler strings have SEMANTIC meaning (a test asserts the exact value). A blind sed is safe
because it renames the production string AND its test assertion IDENTICALLY — they stay matched:

| Straggler string | Production site | Test-assertion site(s) | After sed (both) |
|---|---|---|---|
| Hook status display | `internal/hook/hook.go:30` `return "stagehand (v1)"` | `hook_test.go:232`, `cmd/hook_test.go:150/151` | `"stagecoach (v1)"` |
| Hook script body | `hook/hook.go:120/122`, `hook/script.go:35/37` `` `exec stagehand hook exec "$@"` `` | `hook/script_test.go:15/26/41/100/105`, `cmd/hook_test.go:227/403`, `hooks/runner_test.go:436` | `` `exec stagecoach hook exec "$@"` `` |
| Backup suffix | `integrate/protocol.go:131` `"%s.stagehand-backup.%d"` | `protocol_test.go:357/557`, `cmd/integrate_lazygit_test.go:522/523` | `.stagecoach-backup.` |
| Root command name | `cmd/root.go:121` `Use: "stagehand"` | (cobra Use; no direct string assert seen) | `Use: "stagecoach"` |

**Why the hook-script rename is CORRECT, not just cosmetic:** the binary is `cmd/stagecoach` (M1.T1.S2
renamed the dir). The installed `prepare-commit-msg` hook invokes `exec <binary> hook exec` — it MUST say
`stagecoach` to match the binary that exists. Today's `exec stagehand hook exec` installs a hook pointing at
a binary name that no longer exists → the rename is a NECESSARY fix. (Existing installed hooks from before
the rename will break regardless — that's a deployment/migration concern, not a code concern; the CODE must
emit the current binary name.)

**Already-renamed values whose COMMENTS still say stagehand (sed fixes only the comment):**
- `cmd/integrate_gitalias.go:17` `defaultAliasName = "stagecoach"` (value DONE; comment stale) → sed fixes comment.
- `cmd/integrate_gitalias.go:18` `stagecoachAliasValue = "!stagecoach"` (identifier+value DONE; comment stale).
- `cmd/integrate_lazygit.go:20` `lazygitMarker = "stagecoach-integration"` (DONE; nearby comments stale).
- `hook/hook.go:22` `StatusStagecoach` (identifier DONE; the `String()` return value at :30 is the straggler).

## 4. Categories nominally owned by the sibling P1.M2.T3.S2 (the broad sed subsumes them)

P1.M2.T3.S2 ("session ID prefix, temp dir prefix, bootstrap config template" = Layer 3.4/3.5/3.9) is
PLANNED (not yet running). The broad sed catches those sites too:
- Session ID `"stagehand-"` (multiturn.go:206/208 + generate_multiturn_test.go:218 + render_test.go fixtures) → `"stagecoach-"`.
- Temp prefixes (`stubtest/stubagent`, `signal/stubtest/inttest`, `e2e`, `integrate/diff`, `hooks/hookmsg/hook`) → `stagecoach-*`.
- Bootstrap template `bootstrap.go:236` `# Stagehand configuration file` → `# Stagecoach configuration file`.

**Same direction → no outcome conflict.** If S1 (this task) lands first, it subsumes S2's categories (S2
becomes a verify-only no-op). If the orchestrator re-sequences S2 first, S1's sed is a no-op on those sites.
Either way the end state is identical (zero residue). The PRP flags this explicitly so the orchestrator can
re-sequence if it wants S2 to remain meaningful.

## 5. Overlap with the PARALLEL P1.M2.T2.S2 (.stagehandignore)

The parallel task (running NOW) renames `.stagehandignore` → `.stagecoachignore` at `cmd/root.go:164` +
`ui/verbose.go:101`. The broad sed (`s/stagehand/stagecoach/g`) ALSO converts `.stagehandignore` →
`.stagecoachignore` (the token contains "stagehand"). **Same direction → no conflict.** If both edit those
lines, the result is identical (git auto-resolves or one overwrites identically). Low risk; the zero-residue
gate is unaffected.

## 6. Scope fences (do NOT touch — other tasks own these)

- **Non-`.go` files** (the sed is `--include='*.go'`): `README.md`, `docs/*.md`, `Makefile`, `.goreleaser.yaml`,
  `providers/*.toml`, `.github/workflows/*`, `FUTURE_SPEC.md` → P1.M3 (build/CI) + P1.M4 (docs).
- **`bin/*` + root `stagehand`/`stagecoach` binaries**: compiled build artifacts (rebuild clean from source).
- **Identifiers / import paths / module path**: ALREADY renamed (M1.T1/M1.T2). The sed must NOT touch them —
  verified none remain (all 377 hits are comments/strings).
- **`plan/` artifacts** (this PRP, research, rename_surface_map, prd_snapshot): P1.M5.T1 owns the plan-dir
  bulk rename. Do NOT sed the plan dir.

## 7. The mechanism + verification (deterministic)

```bash
cd /home/dustin/projects/stagehand   # codebase root (module github.com/dustin/stagecoach; on-disk dir name unchanged)
# THE rename (broad, .go-only, two case variants):
grep -rl 'stagehand\|Stagehand' --include='*.go' internal/ cmd/ pkg/ | xargs sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g'
# GATE 1 (zero residue, case-insensitive):
grep -rni 'stagehand' --include='*.go' . | grep -v '.git/' | grep -v '.pi-subagents' | wc -l   # MUST be 0
# GATE 2 (compiles — proves no identifier/import corrupted):
go build ./...
# GATE 3 (semantic strings stayed coordinated — production == test assertions):
go test ./... -count=1
# GATE 4 (format clean):
gofmt -l internal/ cmd/ pkg/
```

GATE 1 = 0 is the contract's OUTPUT. GATE 3 (-count=1, no cache) is the coordination proof — if a production
string and its test assertion diverged (impossible under a uniform sed, but defensive), a test fails here.

## 8. Files touched

ALL `.go` files under `internal/`, `cmd/`, `pkg/` that contain `stagehand`/`Stagehand` (~60 files, 377
occurrences). NO non-`.go` files. NO new files. NO go.mod/go.sum change (content-only string/comment rename).
The codebase is at `/home/dustin/projects/stagehand` (NOT `/home/dustin/projects/stagecoach` — that's the
plan-staging dir; matches the parallel P1.M2.T2.S2 PRP's note).
