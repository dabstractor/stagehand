# Research Note — P1.M1.T1.S3 (git-config resolver keys: stagecoach.tokenLimit / stagecoach.diffContext)

## What this subtask does

Add two int keys to the git-config resolver `loadGitConfig` in `internal/config/git.go`, mirroring the
existing `stagecoach.maxDiffBytes` block exactly, so that:
- `git config stagecoach.tokenLimit N` → `c.TokenLimit` (plain `int`)
- `git config stagecoach.diffContext N` → `c.DiffContext` (`*int`)

S3 touches ONLY `internal/config/git.go` (+ its test). No other file.

## S2 has LANDED — the *int contract is real in the working tree

Verified by grep against the CURRENT working tree (S2 is complete, not just planned):

```
config.go:11:  func intPtr(i int) *int { return &i }                      # ← S2 helper; S3 reuses it
config.go:81:  TokenLimit          int   `toml:"token_limit"`             # plain int (FR3d: 0 = unset)
config.go:82:  DiffContext         *int  `toml:"diff_context"`            # *int — nil ⇒ unset (default 1)
config.go:175: DiffContext:         intPtr(1),                            # Defaults seeds non-nil *1
file.go:53:    DiffContext         *int  `toml:"diff_context"`            # file struct also *int
file.go:226:   if g.DiffContext != nil { c.DiffContext = g.DiffContext }  # materialize guard
```

So S3 implements against the ACTUAL post-S2 state — `Config.DiffContext` is already `*int`. S3 must set
`c.DiffContext = intPtr(n)` (non-nil when found, nil when absent). This is the S2 "DOWNSTREAM HOOK":
"S3 must set `c.DiffContext = intPtr(v)` when stagecoach.diffContext is found (nil when absent)."

## The exact pattern to mirror (git.go:181-186 — the MaxDiffBytes block)

```go
	if v, found, err := gitConfigGet(repoDir, "stagecoach.maxDiffBytes"); err != nil { // camelCase!
		return nil, err
	} else if found {
		if err := parseInt(repoDir, "stagecoach.maxDiffBytes", v, &c.MaxDiffBytes); err != nil {
			return nil, err
		}
	}
```

Helpers (already in git.go): `gitConfigGet(repo, key) (value string, found bool, err error)` and
`parseInt(repo, key, value string, dst *int) error`. The `found` boolean from `gitConfigGet` is the gate
(the `else if found` branch) — a missing key returns `found=false, err=nil` (NOT an error). This is the
gate the contract demands ("use the `found` boolean … NOT the parsed value").

## The crux: DiffContext is *int, so it CANNOT be an exact byte-for-byte mirror

`parseInt(dst *int)` writes through a `*int`. `c.MaxDiffBytes` is `int`, so `&c.MaxDiffBytes` is `*int` ✓.
But `c.DiffContext` is `*int` (S2), so `&c.DiffContext` is `**int` — NOT assignable to `parseInt`'s `dst *int`.

⇒ DiffContext must parse into a LOCAL `var n int` and then wrap: `c.DiffContext = intPtr(n)`.

```go
	if v, found, err := gitConfigGet(repoDir, "stagecoach.diffContext"); err != nil { // camelCase!
		return nil, err
	} else if found {
		var n int
		if err := parseInt(repoDir, "stagecoach.diffContext", v, &n); err != nil {
			return nil, err
		}
		c.DiffContext = intPtr(n) // nil when absent (found=false); non-nil incl. *0 when found
	}
```

This is the ONLY structural deviation from the exact MaxDiffBytes mirror, forced by S2's `*int`. It reuses
`parseInt` (consistent error message format) and `intPtr` (S2's helper). The TokenLimit block IS an exact
mirror (plain int → `&c.TokenLimit` works directly).

## The contract's load-bearing NOTE (do NOT drop an explicit 0)

> Do NOT add a `!= 0` guard that would drop an explicit git-config 0 — use the `found` boolean from
> gitConfigGet to gate (only write when the key is found), NOT the parsed value. This preserves a user's
> explicit `git config stagecoach.diffContext 0`.

Trace showing the design satisfies this:
| git config value | `found` | block runs? | resulting `c.DiffContext` | overlay behavior |
|---|---|---|---|---|
| (unset) | false | no | `nil` | `!= nil` false → skip → inherit lower layer (default *1) ✓ |
| `0` | true | yes, `n=0` | `intPtr(0)` (non-nil, *0) | `!= nil` true → copy → **explicit *0 preserved** ✓ |
| `1` | true | yes, `n=1` | `intPtr(1)` | override *1 ✓ |
| `3` | true | yes, `n=3` | `intPtr(3)` | override *3 ✓ |

The `c.DiffContext = intPtr(n)` is UNCONDITIONAL inside the `found` block — there is NO `if n != 0` value
guard. That is the contract's explicit requirement, and it is what makes `git config stagecoach.diffContext 0`
survive (FR3f: 0 = changed-lines-only is a first-class value). A `!= 0` value guard would drop the explicit
0 (the bug the contract warns against).

## Placement

The four existing int blocks in `loadGitConfig`: maxDiffBytes (181), maxMdLines (188), maxDuplicateRetries
(195), subjectTargetChars (202), then `return c, nil` (210).

**Recommended**: insert the two new blocks immediately after the maxMdLines block (after line 193, before
maxDuplicateRetries at 195) — groups all four diff-capture knobs (maxDiffBytes/maxMdLines/tokenLimit/
diffContext) contiguously, matching the "mirror MaxDiffBytes" spirit. **Alternative** (also acceptable):
append after subjectTargetChars (before `return c, nil` at 210) — purely additive. Either is fine; the
PRP recommends the grouped placement.

## Test idiom (from git_test.go — established helpers)

```go
t.Setenv("HOME", t.TempDir())   // isolate global git config (FINDING E)
repo := t.TempDir()
initRepo(t, repo)               // git init + user.name/user.email
setGitConfig(t, repo, "stagecoach.diffContext", "0")
cfg, err := loadGitConfig(repo) // returns *Config
```

`initRepo` and `setGitConfig` are _test.go helpers in package config (git_test.go:24,46). Naming
convention: `TestLoadGitConfig_<Scenario>`. The existing parse-error test is `TestLoadGitConfig_BadTimeout`
(git_test.go:~203) — mirror it for the new keys.

Required test rows (contract: "unset, =0, =1, =3"):
- **unset** → `cfg.TokenLimit == 0` AND `cfg.DiffContext == nil` (loadGitConfig does NOT apply Defaults;
  nil is what makes overlay skip and inherit the *1 default — asserting nil here is load-bearing).
- **tokenLimit=120000** → `cfg.TokenLimit == 120000`.
- **diffContext=0** → `cfg.DiffContext != nil && *cfg.DiffContext == 0` (THE load-bearing explicit-0 row).
- **diffContext=1** → `*cfg.DiffContext == 1`.
- **diffContext=3** → `*cfg.DiffContext == 3`.
- **(recommended) parse error**: `stagecoach.diffContext=abc` → `loadGitConfig` returns non-nil err; same
  for `stagecoach.tokenLimit=NaN`.

## Scope (explicitly NOT touched)

- `config.go` (S1/S2 own it — fields, Defaults, intPtr; S3 only READS them), `file.go` (S2), `load.go`
  (the overlay flow — unchanged), `bootstrap.go` + `docs/CONFIGURATION.md` (S4 — the user-facing key
  reference), `StagedDiffOptions`/6 call sites (P1.M1.T2), the diff functions (P1.M2+).
- S3 is `internal/config/git.go` + `internal/config/git_test.go` ONLY.

## Validation summary

- `go build ./...` + `go vet ./...` + `gofmt -l .` clean; `go test -race ./...` green.
- New test passes incl. the explicit-0 row (`*cfg.DiffContext == 0`) and the unset row (`cfg.DiffContext == nil`).
- `git diff --stat` → ONLY `internal/config/git.go` + `internal/config/git_test.go`.
- Grep: `grep -n "stagecoach.tokenLimit\|stagecoach.diffContext" internal/config/git.go` → 2 keys present.
