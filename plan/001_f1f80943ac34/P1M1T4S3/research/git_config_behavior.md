# Research: `git config` behavior for Stagecoach (P1.M1.T4.S3)

Empirically verified against the real `git` binary in a temp repo (2026-06-29). These findings are
the factual basis for the S3 PRP. The single most important one (A) **overrides the literal contract
wording** ("auto_stage_all") and FR36 — they name an **invalid** git key.

## FINDING A — 🔴 CRITICAL: git config keys CANNOT contain underscores

`git config stagecoach.auto_stage_all on` → **`error: invalid key: stagecoach.auto_stage_all`** (exit 1
on the WRITE; the matching READ `git config --get stagecoach.auto_stage_all` ALSO errors "invalid key").

Git config key grammar (`git-config` docs): a key is `section.name` (or `section.subsection.name`)
where **section** and **name** may contain only alphanumeric characters and `-` (hyphen). **Underscore
`_` is NOT permitted** in section or name. `subsection` may contain anything, but the dotted form is
what `git config --get` parses, so the leaf name after the final `.` is what matters here — and it
forbids `_`.

Therefore the snake_case names in **FR36** (`stagecoach.auto_stage_all`, `stagecoach.max_diff_bytes`,
…) and the S3 **contract** ("auto_stage_all, etc.") are **literally unusable** in git config. The
authoritative working form is the **PRD §16.3 example**, which uses **camelCase**:

```ini
[stagecoach]
    provider = pi
    model = glm-5.2
    timeout = 90
    autoStageAll = true      # camelCase — VALID (alphanumeric, no underscore)
```

**Verified valid camelCase keys** (set + plain `--get` + `--bool --get` all succeed, exit 0):

`stagecoach.provider`, `stagecoach.model`, `stagecoach.timeout`, `stagecoach.autoStageAll`,
`stagecoach.verbose`, `stagecoach.maxDiffBytes`, `stagecoach.maxMdLines`,
`stagecoach.maxDuplicateRetries`, `stagecoach.subjectTargetChars`, `stagecoach.output`,
`stagecoach.stripCodeFence`.

Single-word keys (`provider`, `model`, `timeout`, `verbose`, `output`) have no `_` and are valid
either way; **all multi-word keys MUST be camelCase** to match §16.3 and to satisfy git's grammar.

**Decision:** S3 reads camelCase keys (the §16.3 form). This is a discovered correction of the
contract/FR36 naming, documented in the PRP as the central design call. The §16.2 TOML file still
uses snake_case (`auto_stage_all`) — that is a DIFFERENT layer (S2's TOML decode) and is unaffected;
only the git-config layer is camelCase.

## FINDING B — exit codes of `git config --get`

- **exit 0** → key found; value on stdout (trailing `\n`).
- **exit 1** → key missing. **This is NOT an error** (core contract point: "Missing keys are not
  errors"). Same exit 1 for `--bool --get` on a missing key (verified).
- exit 2 → usage error (won't happen with well-formed args).
- **No exit 128 observed for READS.** Reading config outside a git repo exits **1** (treated as "not
  found"), because `git config --get` reads system + global config even with no local repo. (Writing
  without a repo exits 128/129, but S3 only reads.) Implication: loadGitConfig does NOT distinguish
  "non-repo" from "no stagecoach keys" — both yield a zero Config, never an error. Defensive code
  still treats `exit != 0 && != 1` as a wrapped error, but it effectively never fires for reads.

## FINDING C — `--bool` normalizes values

`git config --bool --get stagecoach.autoStageAll` returns the canonical literal **`true`** or
**`false`** for any git-boolean input (`true`/`false`, `yes`/`no`, `on`/`off`, `1`/`0`,
case-insensitive). Verified: `autoStageAll = on` → `true`; `stripCodeFence = 1` → `true`. So booleans
MUST be read with `--bool`; plain `--get` would return the raw `on`/`1` and require manual parsing.
`strconv.ParseBool` on the `--bool` output never fails (it's always canonical).

## FINDING D — `timeout` is a bare integer (seconds)

§16.3: `timeout = 90` → `git config --get stagecoach.timeout` returns `"90"` (a bare number). Parse
with `strconv.Atoi` → `time.Duration(n) * time.Second`. A value like `"90s"` would FAIL Atoi (it is
NOT valid here) — that duration-STRING form belongs to the §16.2 TOML file (S2's domain,
`time.ParseDuration`). The two layers spell the duration differently by design: git-config = integer
seconds, TOML = Go duration string. Surface a wrapped error on a non-integer so it fails at LOAD.

## FINDING E — plain `git config --get` reads local + global + system (merged)

No `--local` flag is used (contract: plain `git config --get`). This is the INTENDED behavior — PRD
§16.3: "This composes naturally with the author's existing git commit-pi alias habit and with git
config --local vs --global." A user can set `stagecoach.provider` globally (`--global`) or per-repo
(`--local`), and the merged read picks up whichever applies (local overrides global within git's own
merge). S3 just runs `git -C <repo> config --get <key>` and git does the merge.

## FINDING F — import-cycle reasoning (placement)

`loadGitConfig` returns a `*config.Config`, so its implementation package must either (a) be
`internal/config` itself, or (b) be reachable without a cycle. Placing it in `internal/git` creates a
cycle: `internal/git` would import `internal/config` (for the return type `*Config`) AND S4's
`internal/config.Load()` would import `internal/git` (to call the reader) → **import cycle**.
`internal/git`'s `run()` helper is also UNEXPORTED, unreachable from `internal/config`. The
`internal/git.Git` interface (P1.M1.T2/T3, COMPLETE) has no config-reading method, and adding one
would modify completed work. **Decision:** S3 lives in `internal/config/git.go` and shells out to git
**directly via `os/exec`** — the contract's explicit second option ("reuse internal/git runner OR call
git config directly"). A small self-contained exec helper (LookPath + `-C <repo>` + separate
stdout/stderr buffers + `errors.As(*exec.ExitError)`) mirrors `internal/git.run()`'s proven pattern.
No new external dependency (stdlib `os/exec`, `context` only).

## FINDING G — non-zero overlay limitation is inherited (bool=false can't be forced)

Because `Config` is plain-typed (S1 froze it) and S2's `overlay` copies only NON-ZERO fields, a
git-config `autoStageAll = false` is "found" (exit 0, `--bool` → "false") and S3 sets
`Config.AutoStageAll = false`, but that zero value is indistinguishable from "not set" and is NOT
applied by overlay. This is the SAME documented v1 limitation as the TOML layers (S2). Escape hatches:
env vars (S4, presence-checked) and CLI flags (S4, `flag.Changed`). S3 documents this; it does NOT
retype `Config` to pointers (would break S1 + all consumers). Consistent with S2.
