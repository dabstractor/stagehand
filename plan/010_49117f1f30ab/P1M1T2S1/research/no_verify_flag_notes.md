# Research: --no-verify Flag Definition (P1.M1.T2.S1)

> **Purpose:** Pin the exact edit for registering the `--no-verify` persistent flag in root.go + the
> docs/cli.md row, checked against the live codebase on 2026-07-05. **`flagNoVerify` is ABSENT from
> root.go (genuine add); `go build ./...` succeeds; Config.NoVerify is LANDED (S1 Complete).** The prior
> parallel PRP (P1.M1.T1.S2) writes the loadFlags READER only — it does NOT touch root.go → no conflict.

---

## 1. Environment & Baseline

| Item | Value / Evidence |
|---|---|
| Module | `github.com/dustin/stagecoach`, `go 1.22` |
| Edit targets | `internal/cmd/root.go` (var + BoolVar), `docs/cli.md` (table row) |
| Config.NoVerify | **LANDED** (S1 Complete) — config.go:128-134 (field) + :205 (default false). |
| flagNoVerify today | ABSENT from root.go — `grep flagNoVerify root.go` → none. Genuine add. |
| Baseline | `go build ./...` → OK. |
| Prior PRP (S2) | load.go loadEnv/loadFlags + git.go gitConfigBool (the --no-verify READER). Explicitly says: "The `--no-verify` flag VAR is registered in P1.M1.T2.S1 (root.go); S2 writes only the loadFlags READER." → **no conflict**. |
| Sibling task | P1.M3 (the RunCommitHooks runner that CONSUMES cfg.NoVerify) — future. |

---

## 2. The Three Edits (verified against live source)

### 2.1 root.go — var declaration (after line 100)
Current:
```go
var flagEdit bool     // line 97
var flagPush bool     // line 100
```
Target — add `var flagNoVerify bool` after `flagPush`:
```go
var flagEdit bool
var flagPush bool
var flagNoVerify bool
```

### 2.2 root.go — BoolVar registration (after the --push block, ~line 213)
The --push registration is at lines 206-213 (multi-line help string, ending with `)`). The next line
(214) is `// §15.2 reasoning flags (FR-R6)...`. Insert the --no-verify BoolVar BETWEEN them:
```go
	pf.BoolVar(&flagNoVerify, "no-verify", false,
		"Bypass pre-commit and commit-msg hooks for this commit (mirrors git commit --no-verify; "+
			"prepare-commit-msg and post-commit still run). (env STAGECOACH_NO_VERIFY, git "+
			"stagecoach.no_verify; default false.) (§9.25 FR-V5)")
```
Mirrors --push's help structure: env/git-config/default citation + FR ref.

### 2.3 docs/cli.md — table row (after line 43, the --push row)
Current (line 43-44):
```
| `--push` | bool | false | `STAGECOACH_PUSH` | `stagecoach.push` | Run plain `git push` ... (§9.22 FR-P1) |
| `--planner-provider <name>` | string | "" | ... |
```
Target — insert the --no-verify row BETWEEN --push and --planner-provider:
```
| `--no-verify` | bool | false | `STAGECOACH_NO_VERIFY` | `stagecoach.no_verify` | Bypass `pre-commit` and `commit-msg` hooks for this commit (mirrors `git commit --no-verify`; `prepare-commit-msg` and `post-commit` still run). §9.25, FR-V5. |
```

---

## 3. Decisions Log

| # | Question | Decision | Rationale |
|---|---|---|---|
| D1 | Var placement? | `var flagNoVerify bool` after `var flagPush bool` (line 100). | Groups with push/edit; the contract says "next to flagPush/flagEdit". |
| D2 | BoolVar placement? | After the --push block (~line 213), before the reasoning flags comment (line 214). | The contract: "after the --push registration (~line 212)". |
| D3 | Help text? | The verbatim text from the contract (env/git-config/default citation + FR-V5). | Mirrors --push's help structure exactly. |
| D4 | Docs row placement? | After --push (line 43), before --planner-provider (line 44). | The contract: "after the --push row at line 43". Matches the PRD §15.2 ordering. |
| D5 | Test? | NONE for this task. | The flag is a definition only; the loadFlags reader test (TestLoadFlags_NoVerify) is S2's. The validation is `go build` + `stagecoach --help`. |
| D6 | Direct read? | NEVER — the flag var is read ONLY via `fs.Changed`/`fs.GetBool` in loadFlags (S2). Same discipline as flagPush/flagEdit. | The contract: "never read directly (same discipline as flagPush)." |
