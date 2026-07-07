---
name: "P1.M2.T1.S2 — Rename stagehand.* git config keys to stagecoach.* across all Go files"
description: |
  Part of the stagehand→stagecoach project rename (PRD h2.30). Renames the LOWERCASE `stagehand.*` git-config
  section to `stagecoach.*` in all `.go` files (PRD §9.8 FR36 + §16.3: "Git config keys live under the
  `stagecoach.` section … `stagecoach.provider`, `stagecoach.model`, … per-role `stagecoach.role.<role>.*`").
  The keys appear as ~100 git-config-key references: quoted key literals (the gitConfigGet/gitConfigBool/
  parseInt args + test setGitConfig assertions), root.go help text (`git stagehand.role.planner`), the
  bootstrap.go + cmd/config.go template, error-source strings, and comments — across ~15 .go files. After the
  rename: zero git-config-key `stagehand.` refs remain; the setter↔reader coupling is consistent; the config
  test suite passes with `stagecoach.*` keys. Research: `architecture/` rename surface (the item cites
  rename_surface_map.md Layer 2.2) and `research/git_config_key_rename.md` (the verified-against-live surface
  map with the scope-critical three-category split).

  ⚠️ **THE central design call — RENAME ONLY git-config-key `stagehand.` refs; PRESERVE `.stagehand.toml`
  (filename) AND `pkg/stagehand.` (package-path comments).** `grep 'stagehand\.'` in .go surfaces ~220 hits
  but they split into THREE categories: (A) git-config-key refs (`stagehand.provider`, `(stagehand.*)`,
  `git stagehand.role.planner`) — S2's target, ~100 sites; (B) `.stagehand.toml` filename refs (preceded by
  `.`) — P1.M2.T2.S1's scope, MUST survive; (C) `pkg/stagehand.<thing>` stale package-path COMMENT refs
  (preceded by `/`) — M1 residue, NOT git config keys, out of S2 scope. A naive broad sed `s/stagehand\./stagecoach./g`
  would wrongly rename B (overlapping P1.M2.T2.S1) — FORBIDDEN. See `research/git_config_key_rename.md` §2.

  ⚠️ **THE second design call — the item's narrow sed `s/"stagehand\./"stagecoach./g` is INCOMPLETE.** It
  catches only the 70 quoted-key literals (where the key IS the whole quoted string). It MISSES ~30
  git-config-key refs in LARGER strings/comments: root.go help text (`git stagehand.role.planner` — preceded
  by a space, not a quote), the bootstrap/config template (`git config stagehand.provider`), error-source
  strings (`src = "git config stagehand.provider"`), and free-form comments (`// stagehand.push`). The item
  itself acknowledges a phase-2 manual cleanup ("Run grep ... to find remaining references in comments and
  fix them"). The robust, scope-safe replacement is a SINGLE perl negative-lookbehind pass.

  ⚠️ **THE third design call — use perl `(?<![.\/])stagehand\.` (one pass, provably scope-safe).** The
  negative lookbehind renames `stagehand.` UNLESS preceded by `.` (`.stagehand.toml` → survives) or `/`
  (`pkg/stagehand.` → survives). It catches ALL category-A refs in one atomic pass — quoted literals, help
  text, template, error strings, comments — because none of those are preceded by `.` or `/`. perl is on
  ubuntu-latest (CI) and every dev platform. (Fallback if perl is unavailable: phased seds — quoted-key sed
  + `git stagehand.`/`git config stagehand.`/`(stagehand.*)` anchored seds — then manual edit of the ~15
  free-form comment refs. Same verification.) See `research/git_config_key_rename.md` §3.

  ⚠️ **THE fourth design call — the codebase is at `/home/dustin/projects/stagehand`, NOT `/home/dustin/projects/stagecoach`.**
  The plan-staging cwd (`.../stagecoach`) contains ONLY `plan/`. The actual Go codebase is at
  `/home/dustin/projects/stagehand` (M1 renamed the MODULE PATH to `github.com/dustin/stagecoach` but the
  on-disk directory keeps its name). ALL research/edits/verification target `/home/dustin/projects/stagehand`.
  Verify with `head -1 go.mod` → `module github.com/dustin/stagecoach`.

  ⚠️ **THE fifth design call — the rename MUST be atomic across the setter↔reader coupling.** git.go's
  loadGitConfig READS `"stagehand.provider"`; git_test.go/load_test.go SET `"stagehand.provider"` via
  setGitConfig. The perl pass renames BOTH → setter writes `stagecoach.*`, reader reads `stagecoach.*` → they
  match → `go test ./internal/config/...` passes. Renaming only one side breaks the tests.

  SCOPE: rename category-A `stagehand.*` git-config-key refs to `stagecoach.*` in `--include='*.go'` files
  ONLY. NO `.stagehand.toml` (P1.M2.T2.S1), NO `pkg/stagehand.` comment refs (M1 residue / final audit), NO
  non-.go files (docs/.toml/.yml — P1.M3/P1.M4), NO uppercase `STAGEHAND_` (P1.M2.T1.S1, parallel). INPUT =
  the compiled project from M1 (identifiers already `Stagecoach`; module already `stagecoach`). OUTPUT =
  zero git-config-key `stagehand.` refs; `stagecoach.*` keys set AND read consistently; `go test
  ./internal/config/... -count=1` passes. DOCS: Mode A — docs/configuration.md git-config refs are P1.M4.T1.S2
  (NOT this task; S2 is .go-only).
---

## Goal

**Feature Goal**: Rename every git-config-key `stagehand.*` reference to `stagecoach.*` across all Go source
files so the git-config section matches the stagecoach identity per PRD §9.8 FR36 + §16.3 ("Git config keys
live under the `stagecoach.` section"). This covers the quoted key literals (gitConfigGet/gitConfigBool/
parseInt args + test assertions), the root.go flag help text, the bootstrap.go + cmd/config.go config-init
template, error-source strings, and code comments — ~100 sites across ~15 .go files.

**Deliverable**: A single perl negative-lookbehind pass (`s/(?<![.\/])stagehand\./stagecoach./g`) on all
`*.go` files containing `stagehand.`, followed by a four-gate verification (zero residual git-config-key
refs; `.stagehand.toml` survives; `pkg/stagehand.` survives; config test suite green). No new files; edits only.

**Success Definition**: `grep -rn 'stagehand\.' --include='*.go' . | grep -v '/.git/' | grep -vE '\.stagehand\.|/stagehand\.'`
returns **zero**; `.stagehand.toml` refs are byte-unchanged; `pkg/stagehand.` comment refs are byte-unchanged;
`go build ./...`, `go vet ./...`, `gofmt -l`, `go test ./... -count=1` all green; go.mod/go.sum unchanged;
no non-.go files touched; no uppercase `STAGEHAND_` changed (P1.M2.T1.S1's parallel scope).

## User Persona

**Target User**: Users + CI who set per-repo git config (`git config stagecoach.provider pi`,
`git config stagecoach.role.planner.model zai/glm-5.2`). Transitively PRD §9.8 FR36 + §16.3 (the git-config
section) + §15.2 (flag help text naming the git-config key).

**Use Case**: `git config stagecoach.provider pi && stagehand` (git-config override at layer 4 of FR34) reads
the `stagecoach.provider` key correctly.

**Pain Points Addressed**: the git-config surface matches the new project name; no stale `stagehand.*` keys
remain to confuse users, docs, or the flag help text.

## Why

- **Completes the git-config-key surface rename.** FR36 documents `stagecoach.*`; the code must match. Without
  this, git-config keys are `stagehand.*` while the module, identifiers, and (after S1) env vars say stagecoach.
- **Atomic + safe.** The perl negative-lookbehind is one pass that catches every category-A ref while provably
  preserving the two out-of-scope categories (`.stagehand.toml`, `pkg/stagehand.`). No manual per-site edits
  (which would miss comments/help text).
- **Setter↔reader consistent.** The pass renames both the git.go reader and the test setters atomically → the
  config test suite passes with `stagecoach.*`.
- **Scope-disciplined.** Leaves `.stagehand.toml` for P1.M2.T2.S1 and `pkg/stagehand.` comments for the final
  audit — no overlap with sibling tasks.

## What

Every git-config-key `stagehand.` reference in `.go` files becomes `stagecoach.`. The `.stagehand.toml`
filename and the `pkg/stagehand.` package-path comments are UNCHANGED. Nothing else changes (uppercase env
vars, non-.go files, identifiers, imports are out of scope). The full test suite passes with the new keys.

### Success Criteria

- [ ] `grep -rn 'stagehand\.' --include='*.go' /home/dustin/projects/stagehand | grep -v '/.git/' | grep -vE '\.stagehand\.|/stagehand\.'`
      returns **zero** (all git-config-key refs renamed).
- [ ] `.stagehand.toml` refs survive: `grep -rn '\.stagehand\.toml' --include='*.go' .` is non-empty and
      UNCHANGED in count (P1.M2.T2.S1's scope, untouched).
- [ ] `pkg/stagehand.` comment refs survive: `grep -rn 'pkg/stagehand\.' --include='*.go' .` is UNCHANGED
      (M1 residue, not S2's scope).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l`, `go test ./... -count=1` (esp. `./internal/config/...`)
      all green.
- [ ] go.mod/go.sum unchanged; no non-.go files touched; no uppercase `STAGEHAND_` changed.

## All Needed Context

### Context Completeness Check

_Pass._ A developer with no prior knowledge can implement this from: the codebase location
(`/home/dustin/projects/stagehand`), the three-category split (rename A; leave B and C), the perl
negative-lookbehind one-liner, and the four verification gates. No feature/design knowledge required — this
is a scope-disciplined literal-prefix rename.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/012_963e3918ec08/P1M2T1S2/research/git_config_key_rename.md
  why: the AUTHORITATIVE verified-against-live surface map. §1 = codebase location (.../stagehand, NOT
       .../stagecoach); §2 = the THREE categories (A rename / B .stagehand.toml leave / C pkg/stagehand. leave)
       with the exact file:line sites; §3 = the perl negative-lookbehind (the scope-safe mechanism) + the
       phased-sed fallback; §4 = the four verification gates; §5 = adjacent-task boundaries; §6 = setter↔reader.
  critical: §2 + §3 + §4. The scope-boundary trap: a broad sed would rename `.stagehand.toml` (overlapping
       P1.M2.T2.S1). The perl lookbehind is what makes one-pass scope-safety possible. Verify with the §4 greps.

- file: PRD.md   §9.8 FR36 (h3.24), §16.3 (h3.78), §16.1 layer 4 (h3.76), h2.30 (the rename note)
  why: FR36 is the spec ("Git config keys live under the `stagecoach.` section (`stagecoach.provider`,
       `stagecoach.model`, … per-role `stagecoach.role.<role>.{provider,model,reasoning}`)"). §16.3 shows the
       `git config stagecoach.provider` usage. h2.30 mandates the rename.
  critical: FR36 fixes the canonical key names (stagecoach.*, per-role stagecoach.role.*) — the rename target.

- file: internal/config/git.go   (loadGitConfig — the PRIMARY reader site)
  why: the gitConfigGet/gitConfigBool/parseInt calls read `"stagehand.provider"`, `"stagehand.model"`,
       `"stagehand.timeout"`, `"stagehand.autoStageAll"` (camelCase!), `"stagehand.maxDiffBytes"`, …, plus the
       error string `"git config stagehand.timeout: %w"` (line 152) and a comment (line 213). The perl pass
       renames the prefix → `"stagecoach.provider"` etc. (camelCase suffixes are preserved — only the prefix
       changes).
  pattern: `gitConfigGet(repoDir, "stagehand.<key>")` → `"stagecoach.<key>"`. The perl lookbehind catches the
           quoted literal (preceded by `"`) AND the error-string `config stagehand.` (preceded by space) AND
           the comment.
  gotcha: git.go uses camelCase keys (`stagehand.autoStageAll`); git_test.go:320 uses snake_case
       (`stagehand.max_diff_bytes`); the bootstrap template uses snake_case (`stagehand.auto_stage_all`). The
       prefix-only rename handles ALL three suffix styles identically. Do NOT "normalize" the suffixes — only
       the prefix changes.

- file: internal/config/git_test.go + load_test.go   (the test setter sites)
  why: setGitConfig(t, repo, "stagehand.provider", "pi") etc. The perl pass renames → tests set `stagecoach.*`
       which loadGitConfig now reads. Also: error-message assertions `strings.Contains(err.Error(), "stagehand.timeout")`
       → `"stagecoach.timeout"` (the git.go error string was renamed too, so the assertion still matches).
  gotcha: the assertion at git_test.go:216 `want it to contain 'stagehand.timeout'` references the ERROR
           string text. git.go:152's error becomes `"git config stagecoach.timeout: %w"`, so the assertion
           must become `'stagecoach.timeout'`. The perl pass renames BOTH the error string AND the assertion
           (both contain `stagehand.timeout` not preceded by `.` or `/`) → they still match. ✓

- file: internal/cmd/root.go   (flag help text — 15 sites)
  why: `pf.StringVar(..., "provider", "", "Provider/agent to use (env STAGECOACH_PROVIDER, git stagehand.provider; ...)")`
       and the per-role help `"... (env STAGECOACH_PLANNER_PROVIDER; git stagehand.role.planner)"`. The
       `stagehand.` here is preceded by a space (` stagehand.`) — the narrow sed `"stagehand\.` MISSES it;
       the perl lookbehind catches it (space is not `.` or `/`).
  pattern: `git stagehand.<key>` → `git stagecoach.<key>`; `git stagehand.role.<role>` → `git stagecoach.role.<role>`.

- file: internal/config/bootstrap.go + internal/cmd/config.go   (the config-init template)
  why: the generated config template's comment block: `# git config stagehand.provider pi`, `(stagehand.*)`,
       `git config --get stagehand.<key>`. These are inside Go raw-string literals (backtick) — the perl pass
       operates on bytes regardless of Go syntax → renames within the raw string. The generated config template
       will now document `stagecoach.*` keys.
  gotcha: bootstrap.go:244 ALSO contains `repo-local .stagehand.toml` (the filename, category B) — the lookbehind
           (`stagehand.` preceded by `.`) PRESERVES it. This is the keystone scope-safety proof: the SAME file
           has BOTH a category-A ref (`git config stagehand.provider`, renamed) AND a category-B ref
           (`.stagehand.toml`, preserved) on adjacent lines.

- file: internal/config/load.go + internal/config/config.go   (comments + error-source string)
  why: load.go:212 `src = "git config stagehand.provider"` (the error-source diagnostic); comments at load.go
       :132/188/198/422/430 and config.go:122/130 (`stagehand.push`, `stagehand.noVerify`, `stagehand.commits`,
       etc.). All category-A; the perl lookbehind catches each (none preceded by `.` or `/`).

- docfile: plan/012_963e3918ec08/P1M2T1S1/PRP.md   (the parallel env-var task)
  why: confirms S1 targets UPPERCASE `STAGEHAND_` (case-sensitive, a DIFFERENT substring). S2's lowercase
       `stagehand.` is independent — no content conflict (different case), though both edit some shared files
       (load.go/load_test.go/root.go/bootstrap.go/config.go). S2 does NOT depend on S1 landing first.
  critical: do NOT rename uppercase `STAGEHAND_` here — that's S1. The perl regex is lowercase-only
       (`stagehand\.`), so it cannot touch `STAGEHAND_`.
```

### Current Codebase tree (relevant slice — the ~15 category-A files)

```bash
# Codebase root: /home/dustin/projects/stagehand   (module github.com/dustin/stagecoach; on-disk name unchanged)
internal/config/
  git.go             # loadGitConfig — the PRIMARY reader (gitConfigGet/Bool/parseInt "stagehand.*" literals) + error string + comment  ← EDIT (category A)
  git_test.go        # setGitConfig + gitConfigGet test assertions + error-message checks                                              ← EDIT (category A)
  load_test.go       # setGitConfig assertions +                                                 comment refs                    ← EDIT (category A)
  load.go            # src="git config stagehand.provider" error-source +                          comment refs                    ← EDIT (category A)
  config.go          #                                                                 comment refs (stagehand.push/noVerify)                  ← EDIT (category A)
  bootstrap.go       # config-init template (git config stagehand.*, (stagehand.*)) — BUT .stagehand.toml on adj. lines SURVIVES       ← EDIT (category A only; B preserved)
internal/cmd/
  root.go            # flag help text (git stagehand.provider, git stagehand.role.<role>) — 15 sites                                   ← EDIT (category A)
  config.go          # config-init Long description + template (git config stagehand.*, (stagehand.*))                                 ← EDIT (category A)
  config_test.go     # t.Error("template missing stagehand.provider git-key doc")                                                     ← EDIT (category A)
go.mod / go.sum      # unchanged (module already stagecoach; this is source-content only)
# CATEGORY B (.stagehand.toml) — NOT edited: file.go, file_test.go, + .toml refs in load.go/load_test.go/bootstrap.go/cmd/*_test.go/etc. (P1.M2.T2.S1)
# CATEGORY C (pkg/stagehand. comments) — NOT edited: render.go, reserve.go, generate.go, providers.go, decompose/roles.go (M1 residue)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. The perl pass edits the ~15 category-A .go files IN PLACE. No structural change.
```

### Known Gotchas of our Codebase & Library Quirks

```bash
# CRITICAL (design call #1/#3): RENAME ONLY category A (git-config-key stagehand.). PRESERVE category B
# (.stagehand.toml — preceded by '.') AND category C (pkg/stagehand. — preceded by '/'). The perl
# negative-lookbehind `(?<![.\/])stagehand\.` is the scope-safe mechanism: it cannot match `.stagehand.` or
# `/stagehand.`. Do NOT use a broad sed `s/stagehand\./stagecoach./g` — it would rename `.stagehand.toml`
# (overlapping P1.M2.T2.S1) AND `pkg/stagehand.` (M1 residue). Both are FORBIDDEN overlaps.

# CRITICAL (design call #4): the codebase is at /home/dustin/projects/stagehand (NOT .../stagecoach). The
# plan cwd .../stagecoach has ONLY plan/. Verify: `head -1 /home/dustin/projects/stagehand/go.mod` →
# `module github.com/dustin/stagecoach`. Run ALL commands from /home/dustin/projects/stagehand.

# CRITICAL (design call #2): the item's narrow sed `s/"stagehand\./"stagecoach./g` is INCOMPLETE — it catches
# only the 70 quoted-key literals and MISSES ~30 refs in help text (root.go `git stagehand.role.planner`),
# template (bootstrap.go `git config stagehand.provider`), error strings (load.go:212), and comments. Use the
# perl negative-lookbehind (catches ALL category-A refs in one pass) OR the phased-sed fallback (research §3).

# CRITICAL (design call #5): the rename is atomic across setter↔reader. git.go reads "stagehand.provider";
# tests set "stagehand.provider". The perl pass renames BOTH → consistent. If you rename only one side, tests
# FAIL (set stagecoach.*, read stagehand.* → not found). The perl pass's single-pass scope guarantees this.

# GOTCHA: the perl lookbehind is the SAME file (bootstrap.go) that contains BOTH category A (renamed) AND
# category B (.stagehand.toml, preserved) on adjacent lines. After the pass, verify bootstrap.go shows
# `git config stagecoach.provider` AND `.stagehand.toml` BOTH (the keystone scope-safety proof).

# GOTCHA: key-suffix styles vary — git.go uses camelCase (stagehand.autoStageAll), git_test.go:320 uses
# snake_case (stagehand.max_diff_bytes), the bootstrap template uses snake_case (stagehand.auto_stage_all).
# The prefix-only rename handles all three identically (only `stagehand.` → `stagecoach.`; the suffix is
# untouched). Do NOT "normalize" suffixes — that's a separate concern, not the rename.

# GOTCHA: perl -i edits in place (like sed -i). On BSD/macOS perl is also available (perl is cross-platform,
# unlike GNU-vs-BSD sed). CI (ubuntu) has perl. If perl is somehow unavailable, use the phased-sed fallback
# (research §3) — NOT a broad sed.

# GOTCHA: the error-message assertion at git_test.go:216 (`want it to contain 'stagehand.timeout'`) references
# the git.go:152 error string text. Both are renamed by the pass → the assertion still matches the new
# `'stagecoach.timeout'`. ✓ (No manual sync needed — the pass handles both atomically.)

# GOTCHA: do NOT touch non-.go files (docs/configuration.md, providers/*.toml, .goreleaser.yaml, Makefile,
# .github/workflows/*.yml). Those are P1.M3/P1.M4. The `--include='*.go'` scope handles this. docs/configuration.md
# git-config refs are explicitly P1.M4.T1.S2 (Mode A docs ride with M4, NOT S2).

# GOTCHA: do NOT rename uppercase STAGEHAND_ (P1.M2.T1.S1, parallel). The perl regex is lowercase-only
# (`stagehand\.`), so it cannot touch STAGEHAND_.

# GOTCHA: pkg/stagehand. comment refs (render.go:34, reserve.go:66, generate.go:28/61, providers.go:106/124/137,
# decompose/roles.go) are STALE package-path comments (M1 renamed the actual pkg/stagehand → pkg/stagecoach
# directory + package declarations but missed these comment refs). They are NOT git config keys. LEAVE THEM —
# the final audit (P1.M5.T2.S1) catches residue. The lookbehind (`/stagehand.`) preserves them automatically.
```

## Implementation Blueprint

### Data models and structure

N/A — no types, no data models. A scope-disciplined literal-prefix rename.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: RUN the perl negative-lookbehind pass (the rename)
  - COMMAND (run from /home/dustin/projects/stagehand):
    grep -rl 'stagehand\.' --include='*.go' . | grep -v '/.git/' \
      | xargs perl -pi -e 's/(?<![.\/])stagehand\./stagecoach./g'
  - WHY: `(?<![.\/])stagehand\.` renames every `stagehand.` NOT preceded by `.` or `/` → catches ALL category-A
    refs (quoted literals, help text, template, error strings, comments) while PRESERVING `.stagehand.toml`
    (preceded by `.`) and `pkg/stagehand.` (preceded by `/`). One atomic pass = setter↔reader consistent.
  - FALBACK (if perl unavailable): phased seds —
      (a) sed 's/"stagehand\./"stagecoach./g'                                     # quoted-key literals
      (b) sed 's/git stagehand\./git stagecoach./g'                               # root.go help text
      (c) sed 's/git config stagehand\./git config stagecoach./g'                 # template + error strings
      (d) sed 's/(stagehand\.\*)/(stagecoach.*)/g'                                # (stagehand.*) section shorthand
    then manually edit the ~15 free-form comment refs (load.go:188/198/422/430, config.go:122/130, git.go:213,
    git_test.go:132/136/147/197/216, load_test.go:645/1186/1446/1453/1464, config_test.go:490). See research §3.
  - PORTABILITY: perl is on ubuntu-latest (CI) + all dev platforms (cross-platform, unlike GNU-vs-BSD sed).

Task 2: VERIFY gate 1 — zero residual git-config-key stagehand. refs
  - RUN (from /home/dustin/projects/stagehand):
    grep -rn 'stagehand\.' --include='*.go' . | grep -v '/.git/' | grep -vE '\.stagehand\.|/stagehand\.'
  - EXPECT: empty. (The grep -vE excludes category B `.stagehand.` and category C `/stagehand.`.)
  - IF non-empty: a category-A ref was missed — re-run the perl pass on the file (check for unusual preceding
    chars; the lookbehind covers everything except `.` and `/`).

Task 3: VERIFY gate 2 — .stagehand.toml SURVIVES (P1.M2.T2.S1's scope)
  - RUN: grep -rn '\.stagehand\.toml' --include='*.go' . | grep -v '/.git/'
  - EXPECT: non-empty, UNCHANGED from before the pass (e.g., file.go:127 `return ".stagehand.toml"` intact).
  - IF empty/changed: the lookbehind failed (you used a broad sed) — `.stagehand.toml` was wrongly renamed.
    REVERT and use the perl lookbehind.

Task 4: VERIFY gate 3 — pkg/stagehand. comment refs SURVIVE (M1 residue, not S2's scope)
  - RUN: grep -rn 'pkg/stagehand\.' --include='*.go' . | grep -v '/.git/'
  - EXPECT: non-empty, UNCHANGED (render.go, reserve.go, generate.go, providers.go, decompose/roles.go refs intact).
  - IF empty/changed: the lookbehind failed — pkg-path comments were wrongly renamed. REVERT and use the perl lookbehind.

Task 5: BUILD + TEST
  - RUN: gofmt -l internal/ pkg/ cmd/ (expect clean); go vet ./...; go build ./...;
    go test ./internal/config/... -count=1; go test ./... -count=1.
  - EXPECT: all green. The renamed keys (stagecoach.*) are set by tests AND read by loadGitConfig consistently.
    If a config test FAILS, a `stagehand.` key literal was missed on one side of the setter↔reader pair (re-check
    Task 2) OR the perl pass didn't run on git.go/git_test.go.

Task 6: FINAL scope audit
  - RUN: git diff --stat → confirm only .go files changed (no .md/.toml/.yml/Makefile).
  - RUN: git diff --exit-code go.mod go.sum → unchanged.
  - RUN: grep -rn 'stagehand\.' --include='*.go' . | grep -v '/.git/' | grep -vE '\.stagehand\.|/stagehand\.' → zero.
  - RUN (keystone): grep -n 'stagecoach.provider\|\.stagehand\.toml' internal/config/bootstrap.go → BOTH present
    (git config stagecoach.provider AND .stagehand.toml on adjacent lines — the scope-safety proof).
```

### Implementation Patterns & Key Details

```bash
# The perl negative-lookbehind (the scope-safe one-pass rename):
grep -rl 'stagehand\.' --include='*.go' /home/dustin/projects/stagehand | grep -v '/.git/' \
  | xargs perl -pi -e 's/(?<![.\/])stagehand\./stagecoach./g'
# (?<![.\/]) = "not preceded by . or /" → preserves .stagehand.toml (cat B) AND pkg/stagehand. (cat C).

# The four verification gates (run from /home/dustin/projects/stagehand):
# (1) zero residual git-config-key refs:
grep -rn 'stagehand\.' --include='*.go' . | grep -v '/.git/' | grep -vE '\.stagehand\.|/stagehand\.'   # EMPTY
# (2) .stagehand.toml survives:
grep -rn '\.stagehand\.toml' --include='*.go' . | grep -v '/.git/'   # NON-EMPTY, unchanged
# (3) pkg/stagehand. comment refs survive:
grep -rn 'pkg/stagehand\.' --include='*.go' . | grep -v '/.git/'     # NON-EMPTY, unchanged
# (4) setter↔reader consistent:
go test ./internal/config/... -count=1                              # PASS

# The keystone scope-safety proof (bootstrap.go has BOTH categories on adjacent lines):
grep -n 'stagecoach\.\|\.stagehand\.toml' internal/config/bootstrap.go
# EXPECT both: `git config stagecoach.provider pi` (cat A, renamed) AND `repo-local .stagehand.toml` (cat B, preserved).
```

```bash
# BEFORE/AFTER illustration (bootstrap.go — the keystone file):
#   BEFORE:
#     #   git config stagehand.provider pi          ← cat A (preceded by space)
#     #   repo-local .stagehand.toml                ← cat B (preceded by .)
#   AFTER (perl lookbehind):
#     #   git config stagecoach.provider pi         ← renamed (space ≠ . or /)
#     #   repo-local .stagehand.toml                ← PRESERVED (preceded by .)
# A broad sed would have WRONGLY produced `.stagecoach.toml` — the lookbehind prevents it.
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): NONE — pure source-content rename; module already `stagecoach` (M1). go mod tidy is a no-op.

PACKAGE EDGES: NONE — no import changes (M1 owned imports; Complete). The rename is string-literal/comment content only.

FROZEN / NOT-EDITED:
  - .stagehand.toml filename refs (category B) — P1.M2.T2.S1 ("Rename config file discovery paths"). The lookbehind preserves them.
  - pkg/stagehand. package-path comment refs (category C) — M1 residue (M1 renamed the dir/pkg but missed these
    comments); the final audit P1.M5.T2.S1 catches them. The lookbehind preserves them.
  - Uppercase STAGEHAND_ env vars — P1.M2.T1.S1 (parallel, Implementing). The lowercase perl regex can't touch them.
  - Non-.go files (docs/configuration.md, providers/*.toml, .goreleaser.yaml, Makefile, .github/workflows/*.yml) —
    P1.M3 (build/CI) + P1.M4 (docs). docs/configuration.md git-config refs are explicitly P1.M4.T1.S2 (Mode A).
  - Go identifiers (Stagecoach) — P1.M1.T2.S1 (Complete). git-config keys are string literals, not identifiers.

DOWNSTREAM / RELATED:
  - P1.M2.T1.S1 (parallel): uppercase STAGEHAND_ → STAGECOACH_. Same files, DIFFERENT substring (case). No content
    conflict; a parallel git-edit may race (orchestrator's concern).
  - P1.M2.T2.S1: .stagehand.toml → .stagecoach.toml (category B). S2 LEAVES these for that task.
  - P1.M4.T1.S2: docs/configuration.md git-config refs (the .md twin of this task).
  - P1.M5.T2.S1: final grep audit ("zero stagehand references in tracked files") — catches category C residue + confirms A/B done.

NO DATABASE / NO ROUTES / NO CONFIG CODE CHANGE (the git-config key names are data, not logic).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagehand
gofmt -l internal/ pkg/ cmd/   # expect: empty (the rename is content-only; no structural change)
go vet ./...                    # expect: clean (no broken identifiers — the keys are string literals)
go build ./...                  # expect: success
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Confirm only .go files changed (no .md/.toml/.yml/Makefile):
git diff --name-only | grep -vE '\.go$' && echo "BAD: non-.go file changed" || echo "only .go files changed (good)"
```

### Level 2: Unit Tests (Component Validation) — the setter↔reader consistency gate

```bash
cd /home/dustin/projects/stagehand
go test ./internal/config/... -count=1 -v   # loadGitConfig reads stagecoach.*; git_test/load_test set stagecoach.*
go test ./... -count=1                       # FULL suite — every test that sets/reads a git-config key
# Expected: ALL PASS. A failure means a stagehand. key was missed on one side of a setter↔reader pair (re-check
#   Verification gate 1) OR .stagehand.toml was wrongly renamed (a fixture file path broke — re-check gate 2).
# -count=1 disables test caching (forces re-run with the new stagecoach.* keys).
```

### Level 3: Integration Testing (System Validation) — the four scope gates

```bash
cd /home/dustin/projects/stagehand
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# GATE 1: zero residual git-config-key stagehand. refs:
grep -rn 'stagehand\.' --include='*.go' . | grep -v '/.git/' | grep -vE '\.stagehand\.|/stagehand\.' && echo "BAD: residual cat-A" || echo "zero git-config-key stagehand. (good)"
# GATE 2: .stagehand.toml SURVIVES (P1.M2.T2.S1's scope):
grep -rn '\.stagehand\.toml' --include='*.go' . | grep -v '/.git/' | wc -l   # EXPECT: > 0, unchanged from before
# GATE 3: pkg/stagehand. comment refs SURVIVE (M1 residue):
grep -rn 'pkg/stagehand\.' --include='*.go' . | grep -v '/.git/' | wc -l    # EXPECT: > 0, unchanged from before
# KEYSTONE: bootstrap.go has BOTH a renamed cat-A ref AND a preserved cat-B ref on adjacent lines:
grep -n 'stagecoach\.\|\.stagehand\.toml' internal/config/bootstrap.go
# EXPECT both present (the scope-safety proof).
```

### Level 4: Creative & Domain-Specific Validation

```bash
cd /home/dustin/projects/stagehand
# Git-config-key parity audit (belt-and-suspenders): every key FR36/§16.3 documents has a stagecoach. reader in git.go:
for k in provider model timeout output format locale template autoStageAll verbose stripCodeFence push noVerify maxDiffBytes maxMdLines tokenLimit diffContext maxDuplicateRetries subjectTargetChars; do
  grep -q "stagecoach.$k" internal/config/git.go || echo "MISSING reader: stagecoach.$k"
done
# EXPECT: no MISSING lines (all global keys have stagecoach. readers). (Per-role keys are help-text-only — root.go.)
# Confirm the per-role help text renamed (root.go):
grep -c 'git stagecoach.role' internal/cmd/root.go   # EXPECT: ≥6 (planner/stager/arbiter/message × provider/model/reasoning)
# golangci-lint: `make lint` (project-wide — the rename is content-only; no lint drift).
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `gofmt -l`, `go vet ./...`, `go build ./...`, `go mod tidy` no-op; only .go files changed.
- [ ] Level 2 green: `go test ./internal/config/... -count=1` + `go test ./... -count=1` (setter↔reader consistent).
- [ ] Level 3: GATE 1 zero residual cat-A; GATE 2 `.stagehand.toml` survives; GATE 3 `pkg/stagehand.` survives;
      keystone bootstrap.go shows both `stagecoach.*` AND `.stagehand.toml`.

### Feature Validation

- [ ] Zero git-config-key `stagehand.` refs in .go (`grep ... | grep -vE '\.stagehand\.|/stagehand\.'` empty).
- [ ] git.go reads `stagecoach.*`; git_test/load_test set `stagecoach.*` (consistent).
- [ ] root.go help text says `git stagecoach.provider` / `git stagecoach.role.<role>`.
- [ ] bootstrap.go + cmd/config.go template documents `stagecoach.*` keys (and STILL references `.stagehand.toml`
      — the filename, unchanged).
- [ ] `go test ./...` green with the new keys.

### Code Quality Validation

- [ ] The rename was scope-disciplined: category A renamed; B (`.stagehand.toml`) and C (`pkg/stagehand.`) preserved.
- [ ] Only .go files changed; no .md/.toml/.yml/Makefile.
- [ ] No uppercase `STAGEHAND_` touched (P1.M2.T1.S1's parallel scope).
- [ ] No Go identifiers broken (the keys are string literals; the rename is content-only).
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] No docs edited here (Mode A docs/configuration.md git-config refs are P1.M4.T1.S2's scope).
- [ ] go.mod/go.sum byte-unchanged; no new files.

---

## Anti-Patterns to Avoid

- ❌ Don't use a broad sed `s/stagehand\./stagecoach./g`. It renames `.stagehand.toml` (category B, P1.M2.T2.S1's
  scope) AND `pkg/stagehand.` (category C, M1 residue). Use the perl negative-lookbehind `(?<![.\/])stagehand.`
  which preserves both. The keystone proof: bootstrap.go has cat-A and cat-B on adjacent lines — only the
  lookbehind renames one and preserves the other.
- ❌ Don't trust the item's narrow sed `s/"stagehand\./"stagecoach./g` alone. It catches only the 70 quoted-key
  literals and MISSES ~30 refs in help text (`git stagehand.role.planner`), template (`git config stagehand.*`),
  error strings (`src = "git config stagehand.provider"`), and comments. Use the perl lookbehind (one pass) or
  the phased-sed fallback (research §3).
- ❌ Don't rename `.stagehand.toml` references. They are the config FILENAME (category B), not git-config keys.
  P1.M2.T2.S1 owns the filename rename. The lookbehind (`stagehand.` preceded by `.`) preserves them.
- ❌ Don't rename `pkg/stagehand.` comment references. They are stale package-path COMMENTS (category C), not
  git-config keys. M1 renamed the actual `pkg/stagehand` → `pkg/stagecoach` directory + package declarations
  but missed these comments. The final audit (P1.M5.T2.S1) catches them. The lookbehind (`/stagehand.`) preserves them.
- ❌ Don't work in `/home/dustin/projects/stagecoach` — that's the plan-staging dir (only `plan/`). The codebase
  is at `/home/dustin/projects/stagehand` (module already `github.com/dustin/stagecoach`). Verify with
  `head -1 go.mod`.
- ❌ Don't rename uppercase `STAGEHAND_` (env vars). That's P1.M2.T1.S1 (parallel). The perl regex is
  lowercase-only (`stagehand\.`); it cannot touch `STAGEHAND_`.
- ❌ Don't touch non-.go files (docs/configuration.md, providers/*.toml, .goreleaser.yaml, Makefile). Those are
  P1.M3/P1.M4. The `--include='*.go'` scope handles it. docs/configuration.md git-config refs are P1.M4.T1.S2.
- ❌ Don't "normalize" key suffixes. git.go uses camelCase (`autoStageAll`), tests use snake_case (`max_diff_bytes`),
  the template uses snake_case (`auto_stage_all`). The prefix-only rename handles all three identically. Do NOT
  change the suffix — that's a separate concern (and would break the setter↔reader pairing).
- ❌ Don't manually edit only git.go. The rename is a SET (git.go reader + git_test/load_test setters + root.go
  help + template + comments). A manual per-file approach WILL miss comments/help. The perl pass is atomic.
- ❌ Don't change go.mod/go.sum. This is a source-content rename (module already `stagecoach`); no dep change.
- ❌ Don't skip the four verification gates. GATE 1 (zero cat-A) + GATE 2 (`.stagehand.toml` survives) + GATE 3
  (`pkg/stagehand.` survives) + the config test suite together PROVE the rename was both complete AND
  scope-disciplined. Skipping any leaves a silent overlap with a sibling task.
