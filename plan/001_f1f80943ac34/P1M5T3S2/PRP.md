---
name: "P1.M5.T3.S2 — goreleaser config (archives, Homebrew, AUR, Scoop, checksums, changelog) + release workflow"
description: |

  Ship `.goreleaser.yaml` (NEW) + `.github/workflows/release.yml` (NEW) — a goreleaser v2 release
  pipeline (PRD §21.2) that, on every pushed `v*` tag, cross-compiles `cmd/stagecoach` to the 6
  `linux/darwin/windows × amd64/arm64` targets (PRD §21.1/G9), produces per-OS archives, a checksums
  file, a sorted changelog, a GitHub Release, a Homebrew formula pushed to `dustin/homebrew-tap`, a
  Scoop manifest pushed to `dustin/scoop-bucket`, and (best-effort) an AUR `stagecoach-bin` package.
  `go install github.com/dustin/stagecoach/cmd/stagecoach@latest` works from the tagged commit (PRD
  §21.2/§21.3) because the module path in `go.mod` matches the release repo — a release-time
  prerequisite called out below.

  CONTRACT (P1.M5.T3.S2, verbatim):
    1. RESEARCH NOTE: "PRD §21.2 — goreleaser produces: archives+binaries for linux/darwin/windows ×
       amd64/arm64; Homebrew formula to dustin/homebrew-tap; AUR stagecoach+stagecoach-bin; Scoop
       manifest; checksums+changelog. §21.1 — version via -ldflags. §21.3 install paths."
    2. INPUT: "Makefile from P1.M1.T1.S2."
    3. LOGIC: "Create .goreleaser.yaml with: build targets (6 combos), ldflags
       (-X main.version={{.Version}}), archive format per OS, brew formula (tap: dustin/homebrew-tap),
       scoop bucket, checksum, changelog. Add a release workflow .github/workflows/release.yml
       triggered on tag. Mock: validate with `goreleaser check` and `goreleaser release --snapshot --clean`."
    4. OUTPUT: "A goreleaser config that produces all distribution artifacts on tag."
    5. DOCS: "none — release config. Install instructions go in README (P1.M5.T4)."

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - `.github/workflows/ci.yml` + `.golangci.yml` → S1 (P1.M5.T3.S1). ci.yml triggers ONLY on
      push(main)+PR. S2's release.yml triggers ONLY on `push: tags: ['v*']`. No overlap. S2 must NOT
      add a tags trigger or a goreleaser step to ci.yml, and must NOT edit .golangci.yml.
    - `Makefile` → UNCHANGED. The Makefile already has `VERSION ?= dev` and
      `LDFLAGS := -X main.version=$(VERSION)` (P1.M1.T1.S2). goreleaser does NOT call `make build`;
      it runs its own `builds:` with its own ldflags. The two are independent; S2 does not touch the
      Makefile. (They agree on the `-X main.version=` symbol because `cmd/stagecoach/main.go` declares
      `var version = "dev"`.)
    - `cmd/stagecoach/main.go` → UNCHANGED. It already declares `var version = "dev"` and sets
      `cmd.Version = version` (cobra `--version`). ldflags injection already works.
    - `go.mod` / `go.sum` → UNCHANGED (module path `github.com/dustin/stagecoach` is the source of
      truth for `go install`).
    - `install.sh` (PRD §21.3 `curl | sh` one-liner) + README install section → P1.M5.T4. OUT of scope.
    - From-source `stagecoach` AUR package → OUT of scope (PRD: "possibly community"; see "AUR
      decision" below — only `stagecoach-bin` via the native pipe is attempted).
    - `PRD.md`, `tasks.json`, `.gitignore` → READ-ONLY (`.gitignore` already ignores `/dist/`,
      `/bin/`, `*.exe`, `coverage.out`; no change needed).

  DELIVERABLE (TWO new files, NO edits to existing files):
    CREATE .goreleaser.yaml            # goreleaser v2: builds/archives/checksum/changelog/release/brews/scoops/aurs
    CREATE .github/workflows/release.yml  # on tag 'v*': checkout(fetch-depth:0) → setup-go → goreleaser-action

  SUCCESS: `goreleaser check` exits 0; `goreleaser release --snapshot --clean` builds 6 archives +
  `*_checksums.txt` into `dist/` WITHOUT publishing; `dist/` contains exactly the 6 OS×arch archives
  (tar.gz for linux/darwin, zip for windows) named `stagecoach_<v>_<os>_<arch>.<ext>`; a built binary
  reports the injected version (not `dev`) via `--version`; `release.yml` is valid (actionlint clean);
  `git status --short` shows only the 2 new files.

---

## Goal

**Feature Goal**: Give Stagecoach a goreleaser v2 release pipeline (PRD §21.2 / G9) that turns a pushed
`v*` git tag into the full distribution artifact set: 6 cross-compiled static binaries
(`linux/darwin/windows × amd64/arm64`), per-OS archives, a SHA256 checksums file, a sorted changelog,
a GitHub Release, a Homebrew formula (pushed to `dustin/homebrew-tap`), a Scoop manifest (pushed to
`dustin/scoop-bucket`), and a best-effort AUR `stagecoach-bin`. Version is injected via
`-ldflags "-X main.version={{.Version}}"` (PRD §21.1), wiring into the existing `var version` in
`cmd/stagecoach/main.go` and cobra's `--version`.

**Deliverable**: Two new files at repo root — `.goreleaser.yaml` (goreleaser v2 config) and
`.github/workflows/release.yml` (the tag-triggered workflow that runs `goreleaser release --clean`).
No edits to any existing file (Makefile, main.go, go.mod, ci.yml all untouched — the wiring they need
already exists).

**Success Definition**:
- `.goreleaser.yaml` + `.github/workflows/release.yml` exist; `goreleaser check` exits 0.
- `goreleaser release --snapshot --clean` succeeds and `dist/` contains:
  - exactly 6 archives: `stagecoach_<v>_linux_amd64.tar.gz`, `…_linux_arm64.tar.gz`,
    `…_darwin_amd64.tar.gz`, `…_darwin_arm64.tar.gz`, `…_windows_amd64.zip`, `…_windows_arm64.zip`;
  - `stagecoach_<v>_checksums.txt`;
  - a changelog file; and 6 binaries (one per target).
- A built binary (`dist/…/stagecoach` from the snapshot, or `goreleaser build --single-target --snapshot`)
  reports the injected version (e.g. `0.0.0-next` / `0.0.0-SNAPSHOT-…`), NOT `dev`, via `--version`.
  (Snapshot versions are synthetic — see "Known Gotchas"; the point is ldflags is applied.)
- `actionlint .github/workflows/release.yml` is clean (or YAML parses as fallback).
- `release.yml` triggers ONLY on `push: tags: ['v*']`; ci.yml is unchanged.
- `git status --short` shows ONLY the 2 new files.

## User Persona

**Target User**: the **release engineer / maintainer** who cuts a release by `git tag vX.Y.Z && git push
--tags`, and every **end user** who then installs via `brew install dustin/tap/stagecoach`,
`scoop install dustin/stagecoach`, `go install github.com/dustin/stagecoach/cmd/stagecoach@latest`, or by
downloading a binary from the GitHub Release (PRD §21.3).

**Use Case**: A maintainer finishes a milestone, runs `git tag v1.0.0 && git push origin v1.0.0`. Within
minutes the release workflow cross-compiles all 6 targets, uploads archives + checksums + changelog to a
GitHub Release, and updates the Homebrew tap + Scoop bucket + AUR. Users on macOS/Linux/Windows ×
amd64/arm64 can immediately `brew`/`scoop`/`go install`/download the new version.

**Pain Points Addressed**: Today there is NO release pipeline (`.goreleaser.yaml` and `.github/workflows/
release.yml` do not exist), so cutting a release would mean hand-building 6 binaries, hand-zipping,
hand-writing a Homebrew formula + Scoop manifest + checksums, and hand-uploading. This is error-prone
and repetitive. goreleaser automates all of it deterministically from one config.

## Why

- **Realizes PRD §21.2 + §21.1 + G9.** §21.2 mandates goreleaser produce archives+binaries for the 6
  OS×arch combos, the Homebrew formula to `dustin/homebrew-tap`, AUR packages, the Scoop manifest, and
  checksums+changelog. §21.1 mandates version via `-ldflags "-X main.version=…"`. G9 mandates the
  distribution channels. This task is that realization.
- **Completes the "release on tag via goreleaser" half of PRD §20.4.** S1 (P1.M5.T3.S1) shipped the CI
  half (`ci.yml`, push(main)+PR). S2 ships the release half (`release.yml`, tag-triggered). Together
  they are §20.4's "build+test matrix + release on tag via goreleaser".
- **Makes `go install …@latest` work (PRD §21.3).** A tagged commit + the matching module path
  (`github.com/dustin/stagecoach`) lets `go install` resolve. goreleaser's job is the binaries + the
  GitHub Release; `go install` works automatically once a tag exists on a fetchable repo (see the
  namespace gotcha — this is a release-time prerequisite, not extra code).
- **Deterministic, reproducible releases.** One config = every release produces identical artifact
  shapes. The snapshot validation gate lets us prove the config is correct WITHOUT publishing.
- **Scope discipline**: S2 ships ONLY `.goreleaser.yaml` + `release.yml`. CI is S1; the Makefile
  coverage gate is S3; the README/install docs are P1.M5.T4.

## What

A two-file release addition (full content in "Implementation Blueprint"):

1. **`.goreleaser.yaml`** (goreleaser **v2**) — `version: 2`. Sections:
   - `project_name: stagecoach`.
   - `builds:` — one build, `id: stagecoach`, `main: ./cmd/stagecoach`, `binary: stagecoach`,
     `env: [CGO_ENABLED=0]` (static cross-compile), `goos: [linux, darwin, windows]`,
     `goarch: [amd64, arm64]` (→ 6 combos), `ldflags: [-s -w -X main.version={{.Version}}]`.
   - `archives:` — `name_template: stagecoach_{{.Version}}_{{.Os}}_{{.Arch}}`, `formats: [tar.gz]`,
     `format_overrides: [{goos: windows, formats: [zip]}]` (windows → zip; others → tar.gz).
   - `checksum:` — `name_template: stagecoach_{{.Version}}_checksums.txt`, `algorithm: sha256`.
   - `changelog:` — `sort: asc`, `filters.exclude` drops `docs:`/`test:`/`chore:`/`ci:` + merge commits.
   - `release:` — `github.owner: dustin`, `github.name: stagecoach`, `draft: false`, `prerelease: auto`
     (explicit owner — see namespace gotcha).
   - `brews:` — formula pushed to repo `dustin/homebrew-tap` via `${HOMEBREW_TAP_GITHUB_TOKEN}`;
     `install: bin.install "stagecoach"`; `test: system "#{bin}/stagecoach", "--version"`.
   - `scoops:` — manifest pushed to repo `dustin/scoop-bucket` via `${SCOOP_BUCKET_GITHUB_TOKEN}`.
   - `aurs:` (BEST-EFFORT) — `stagecoach-bin` pushed over SSH via `${AUR_SSH_PRIVATE_KEY}`; from-source
     `stagecoach` is OUT of scope (manual/community PKGBUILD per PRD wording).
2. **`.github/workflows/release.yml`** — `on: push: tags: ['v*']`; `permissions: contents: write`; one
   job: `checkout@v4` (`fetch-depth: 0` — REQUIRED for changelog), `setup-go@v5` (`go-version-file:
   go.mod`), `goreleaser-action@v6` (`args: release --clean`) with `GITHUB_TOKEN` (release) +
   `HOMEBREW_TAP_GITHUB_TOKEN` + `SCOOP_BUCKET_GITHUB_TOKEN` + `AUR_SSH_PRIVATE_KEY` secrets.

### Success Criteria

- [ ] `.goreleaser.yaml` + `.github/workflows/release.yml` exist at repo root; no existing file changed.
- [ ] `goreleaser check` exits 0 (config schema valid — install goreleaser if needed; see Level 1).
- [ ] `goreleaser release --snapshot --clean` succeeds and `dist/` holds exactly the 6 archives
      (`stagecoach_<v>_{linux,darwin,windows}_{amd64,arm64}.{tar.gz,zip}` — windows = zip) +
      `stagecoach_<v>_checksums.txt` + a changelog. **Nothing is published** (snapshot guarantee).
- [ ] A built binary's `--version` prints the injected (snapshot) version, not `dev` (proves ldflags).
- [ ] `actionlint .github/workflows/release.yml` exits 0 (or YAML parses via python).
- [ ] `.goreleaser.yaml` has `version: 2`, `builds[0].env` includes `CGO_ENABLED=0`,
      `goos: [linux,darwin,windows]`, `goarch: [amd64,arm64]`, and ldflags `-X main.version={{.Version}}`.
- [ ] `release.yml` triggers ONLY on `push: tags: ['v*']`; uses `fetch-depth: 0`; `goreleaser-action@v6`.
- [ ] The Makefile, main.go, go.mod, ci.yml, .golangci.yml are unchanged (`git status --short` = 2 files).

## All Needed Context

### Context Completeness Check

_Pass._ An author who has never seen this repo can implement this from: the exact copy-pasteable
`.goreleaser.yaml` + `release.yml` (§"Implementation Blueprint"); the verified facts about the existing
version wiring (main.go `var version = "dev"`, Makefile `LDFLAGS`, cobra `--version`); the goreleaser v2
schema reference (§"Documentation & References"); the namespace-discrepancy decision tree + AUR decision
(§"Known Gotchas"); and the validation commands (§"Validation Loop"). The genuinely uncertain inputs
(current goreleaser minor version's exact `formats` vs `format` spelling, AUR `private_key` field name)
are each given an explicit `goreleaser check` gate + sanctioned fallback, so a single implementation
pass reaches green.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M5T3S2/research/goreleaser_research.md
  why: THE decisive doc. v2 schema + plural keys (brews/scoops/aurs + repository); the 6-combo build;
       CGO_ENABLED=0 requirement; {{.Version}} strips leading v; AUR native pipe (stagecoach-bin only,
       SSH not PAT); release.github auto-detect vs explicit owner; release.yml canonical pattern;
       snapshot-guarantees-no-publish; --rm-dist → --clean; replacements/rlcp removed; v1→v2 cheat sheet.
  critical: §11 (pitfalls: CGO, universal-binaries-deprecated, --clean, replacements removed),
            §6 (AUR verdict + fallback), §8 (explicit owner wins over auto-detect), §15 (v1→v2 renames).

- file: cmd/stagecoach/main.go   (READ only — the ldflags TARGET)
  why: declares `var version = "dev"` (line ~16) and `cmd.Version = version`. This is the exact symbol
       goreleaser's `-X main.version={{.Version}}` sets. Already wired; S2 changes nothing here.
  pattern: ldflags `-X <pkg>.<symbol>=<value>` where <pkg> is the main package = `main` (the binary's
           main), <symbol> = `version`. Confirmed: `main.version` matches the Makefile's LDFLAGS.

- file: Makefile   (P1.M1.T1.S2 — READ only; goreleaser is INDEPENDENT of it)
  section: `VERSION ?= dev`, `LDFLAGS := -X main.version=$(VERSION)`, `MAIN_PKG := ./cmd/stagecoach`,
           `BIN := bin/stagecoach`, `clean: rm -rf bin/ coverage.out dist/`.
  why: confirms (a) the exact ldflags symbol `main.version`, (b) the main package path `./cmd/stagecoach`,
       (c) the binary name `stagecoach`, (d) `dist/` is already a clean target (and .gitignore ignores it).
  gotcha: goreleaser does NOT call `make build`; it runs its OWN `builds:`. The two share the ldflags
          symbol by coincidence-of-correctness (both set `main.version`), not by coupling. `make build`
          is for local dev; goreleaser is for release. Do NOT make goreleaser shell out to make.

- file: go.mod   (READ only)
  why: `module github.com/dustin/stagecoach` is the source-of-truth identity for `go install` (PRD §21.3)
       and for goreleaser template var `.ModulePath`. `go 1.22` is the toolchain floor; `release.yml`
       pins Go via `go-version-file: go.mod` so CI matches the module. Deps are tiny (cobra, pflag,
       go-toml/v2, mousetrap) → static, no-CGO build is clean.

- file: .gitignore   (READ only — do NOT edit)
  why: already ignores `/dist/` (goreleaser output), `/bin/`, `*.exe`, `coverage.out`. Confirms S2 must
       NOT touch .gitignore — goreleaser artifacts won't be committed by accident.

- file: plan/001_f1f80943ac34/P1M5T3S1/PRP.md   (READ only — the PARALLEL sibling; treat as CONTRACT)
  why: S1 creates .github/workflows/ci.yml + .golangci.yml, triggers push(main)+PR ONLY, NO tags/release.
       Confirms S2 owns release.yml + .goreleaser.yaml with no overlap. S1's repo-root conventions
       (.github/workflows/*.yml) are what release.yml must follow.

# --- goreleaser v2 docs (primary sources; open the AUR + archives pages first to confirm field names) ---
- url: https://goreleaser.com/customization/
  why: index of all sections (builds, archives, brews, scoops, aurs, checksum, changelog, release).
- url: https://goreleaser.com/customization/builds/
  why: goos/goarch/env(CGO_ENABLED)/ldflags/main/binary/id. The 6-combo build section.
- url: https://goreleaser.com/customization/archive/
  why: name_template, formats vs format (plural is current v2), format_overrides. CONFIRM `formats` here.
- url: https://goreleaser.com/customization/homebrew/
  why: `brews:` + `repository:` (owner/name/token), homepage, install, test, caveats. NOTE: config key is
       `brews:` (plural) but URL slug is `homebrew`.
- url: https://goreleaser.com/customization/scoop/
  why: `scoops:` + `repository:`. Same plural-key pattern as brews.
- url: https://goreleaser.com/customization/aur/   # OPEN FIRST for AUR — highest uncertainty
  why: confirms `aurs:` plural, stable-vs-experimental, the SSH `private_key` field name, provides/conflicts.
- url: https://goreleaser.com/customization/checksum/
  why: name_template + algorithm (sha256 default).
- url: https://goreleaser.com/customization/changelog/
  why: sort (asc), filters.exclude regexes, optional groups.
- url: https://goreleaser.com/customization/release/
  why: github.owner/name, draft, prerelease (auto). CONFIRMS explicit owner WINS over git-remote auto-detect.
- url: https://goreleaser.com/customization/templates/
  why: the template variable table — .Version (no leading v), .Tag, .Os, .Arch, .ProjectName, .ArtifactName.
- url: https://github.com/goreleaser/goreleaser-action
  why: the action's inputs — `version` (goreleaser binary) + `args` (`release --clean`). v6 current.
- url: https://goreleaser.com/install/
  why: install goreleaser for local validation: `go install github.com/goreleaser/goreleaser/v2@latest`
       (NOTE the /v2 module path) or `brew install goreleaser`.

# --- PRD (authoritative spec) ---
- doc: PRD.md §21.1 (Makefile build + version via ldflags), §21.2 (goreleaser outputs + targets), §21.3
       (install paths incl. `go install github.com/dustin/stagecoach/cmd/stagecoach@latest`), §21.4
       (semver), §20.4 (release on tag via goreleaser), G9 (distribution channels), line 884
       (`.goreleaser.yaml` in the package-layout tree).
```

### Current Codebase tree (relevant slice)

```bash
.goreleaser.yaml               # ← does NOT exist yet (greenfield) — NEW (this task)
.github/                       # ← created by S1 (in parallel). S2 ADDS release.yml alongside ci.yml.
  workflows/
    ci.yml                     # S1 — push(main)+PR ONLY. UNCHANGED by S2.
    release.yml                # ← NEW (this task) — push: tags ['v*'] ONLY.
.golangci.yml                  # S1. UNCHANGED.
Makefile                       # P1.M1.T1.S2 — VERSION/LDFLAGS/MAIN_PKG. UNCHANGED (goreleaser is independent).
go.mod / go.sum                # module github.com/dustin/stagecoach; go 1.22. UNCHANGED.
cmd/stagecoach/main.go          # var version="dev"; cmd.Version=version. UNCHANGED (ldflags target, already wired).
.gitignore                     # already ignores /dist/ /bin/ *.exe coverage.out. UNCHANGED.
# (S3 will later add a `make coverage-gate` target; P1.M5.T4 will add README + install docs. Neither exists yet.)
```

### Desired Codebase tree with files to be added

```bash
.goreleaser.yaml               # NEW — goreleaser v2 config (builds/archives/checksum/changelog/release/brews/scoops/aurs).
.github/workflows/release.yml  # NEW — on tag 'v*': checkout(fetch-depth:0) → setup-go → goreleaser-action.
# ALL other files UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```yaml
# CRITICAL (#1) — OWNER NAMESPACE: git remote ≠ PRD/go.mod namespace.
#   ACTUAL git remote:  git@github.com:dabstractor/stagecoach   (run `git remote -v` to confirm)
#   PRD §21.2/§21.3 + go.mod module path: github.com/dustin/stagecoach
#   go.mod `module github.com/dustin/stagecoach` is the SOURCE OF TRUTH for `go install` (Go fetches the
#   module from its declared path). So the INTENDED canonical home is `github.com/dustin/stagecoach`.
#   DECISION: the config below uses `dustin/stagecoach` + `dustin/homebrew-tap` + `dustin/scoop-bucket`
#   everywhere (matches PRD contract + go.mod + all PRD §21.3 install commands). For a REAL release to
#   publish, the repo must be reachable AT github.com/dustin/stagecoach (rename/transfer the current
#   `dabstractor/stagecoach`, OR set up a `dustin/stagecoach` that the module path resolves to).
#   WHY THIS IS OK TO SHIP NOW: `goreleaser check` + `goreleaser release --snapshot --clean` publish
#   NOTHING, so the config is FULLY VALIDATABLE today regardless of namespace. The namespace is a
#   release-time PREREQUISITE (a human action), not S2 code. Do NOT silently switch the config to
#   `dabstractor` — that would contradict the go.mod module path and break `go install`. If the project
#   decides to adopt `dabstractor` permanently, that's a SEPARATE change touching go.mod + PRD §21.3 +
#   every owner ref — out of S2's scope. RECORD the chosen owner in the validation notes.

# CRITICAL (#2) — CGO MUST BE OFF for cross-compilation. Without `env: [CGO_ENABLED=0]` in `builds:`,
# the linux/darwin/windows × arm64 cross-builds fail (no C cross-compiler on the CI runner). This is
# the #1 cause of broken goreleaser cross-compile. stagecoach uses only pure-Go deps (cobra, pflag,
# go-toml/v2) → CGO off is safe and produces static binaries. Set it.

# CRITICAL (#3) — goreleaser is NOT installed on the authoring machine (verified: `which goreleaser`
# → not found). The implementer MUST install it to run the validation gates:
#   go install github.com/goreleaser/goreleaser/v2@latest    (NOTE the /v2 module path for v2)
#   (or: brew install goreleaser ; or download from goreleaser.com/install/)
# Without this, `goreleaser check` cannot run and the config is UNVALIDATED — do not ship blind.

# CRITICAL (#4) — AUR is BEST-EFFORT and the riskiest section. goreleaser's native `aurs:` pipe produces
# only `stagecoach-bin` (prebuilt-binary PKGBUILD), pushes over SSH (NOT a GitHub PAT), and some field
# names (private_key) carry verification uncertainty. The PRD says "AUR stagecoach + stagecoach-bin (via a
# maintained PKGBUILD; possibly community)" and the contract LOGIC bullets do NOT list AUR (only build
# targets, ldflags, archive, brew, scoop, checksum, changelog). DECISION: include the native `aurs:`
# block for `stagecoach-bin`; treat from-source `stagecoach` as OUT of scope (manual/community PKGBUILD,
# matching PRD's "possibly community"). FALLBACK: if `goreleaser check` rejects any `aurs:` field name
# (private_key etc.) and the docs can't resolve it quickly, COMMENT OUT the entire `aurs:` block and
# ship brew+scoop+checksum+changelog+archives (the core contract). Document the deferral in the commit.
# AUR does NOT block shipping the rest.

# GOTCHA — `{{.Version}}` STRIPS the leading "v". Tag `v1.0.0` → `{{.Version}}` = `1.0.0`, so
# `-X main.version=1.0.0` and `stagecoach --version` prints `stagecoach version 1.0.0` (cobra's default
# template is `{{.Name}} version {{.Version}}`). PRD §21.4 uses `v1.0.0` semver. If you want the
# `v`-prefixed string in --version, use `{{.Tag}}` instead. The contract says `{{.Version}}` (no v) —
# FOLLOW THE CONTRACT (use {{.Version}}). This is the conventional choice (ripgrep/bat/gh all print the
# no-v form). cobra already prints "version " before it, so `stagecoach version 1.0.0` reads naturally.

# GOTCHA — goreleaser does NOT call `make build`. It has its own `builds:` section that runs `go build`
# directly with its own ldflags/env. The Makefile's `VERSION`/`LDFLAGS` are for LOCAL `make build` only.
# Both set `main.version` (they agree by construction), but they are not coupled. Do NOT add a
# `before:` hook that runs `make`; do NOT assume goreleaser inherits the Makefile's VERSION.

# GOTCHA — `--rm-dist` is GONE in v2. Use `--clean` (`goreleaser release --clean`,
# `goreleaser release --snapshot --clean`). The old flag errors.

# GOTCHA — `replacements:` / `rlcp:` are REMOVED in v2. Do NOT add them (goreleaser check errors).
# Use `name_template` with `{{.Os}}`/`{{.Arch}}` (already lowercased) — the config below does this.

# GOTCHA — v2 uses PLURAL section keys + `repository:`: `brews:`/`scoops:`/`aurs:` (NOT brew/scoop/aur)
# and `repository: {owner, name, token}` (NOT tap:/bucket:). The config below uses the v2 forms.

# GOTCHA — `formats:` vs `format:` (archives). Current v2 prefers the plural LIST `formats: [tar.gz]`
# (and `format_overrides[].formats: [zip]`); the singular `format:` may still be accepted as a
# deprecated alias. USE `formats:` (plural) per the latest docs; if `goreleaser check` rejects it on
# the installed minor, fall back to singular `format: tar.gz` / `format_overrides[].format: zip`.
# (Decision tree in "formats-vs-format decision" below.)

# GOTCHA — macOS UNIVERSAL binaries are DEPRECATED/REMOVED. We want SEPARATE amd64/arm64 archives
# (that's what listing both in `goarch:` yields). Do NOT add any `universal_binaries:`/lipo step.

# GOTCHA — `fetch-depth: 0` in release.yml is MANDATORY. A shallow clone truncates the changelog to
# the last commit. `actions/checkout@v4` defaults to fetch-depth 1 — you MUST override to 0.

# GOTCHA — `${{ secrets.GITHUB_TOKEN }}` (auto-provided) can create the GitHub Release in
# `dustin/stagecoach` but CANNOT push to OTHER repos (`dustin/homebrew-tap`, `dustin/scoop-bucket`).
# Use SEPARATE fine-grained PATs (`contents: write` on each target repo) named HOMEBREW_TAP_GITHUB_TOKEN
# / SCOOP_BUCKET_GITHUB_TOKEN, and an SSH key (AUR_SSH_PRIVATE_KEY) for AUR. These secrets are a
# release-time setup step (Settings → Secrets); their ABSENCE only breaks a REAL release, NOT snapshot.

# GOTCHA — snapshot versions are SYNTHETIC. `goreleaser release --snapshot` builds with a fake version
# like `0.0.0-next`/`0.0.0-SNAPSHOT-<sha>`. So `--version` won't print a real semver in snapshot — it
# prints the synthetic one. That's CORRECT and proves ldflags is applied (the key check is "not `dev`").

# GOTCHA — AUR push is over SSH to aur.archlinux.org, NOT a GitHub PAT. Register an SSH key on the AUR
# account; store the PRIVATE key as secret AUR_SSH_PRIVATE_KEY. (If you skip AUR per the fallback, this
# secret is unused.)

# GOTCHA — goreleaser-action's `version:` field (the BINARY version, e.g. `latest` or `~> v2`) is
# DIFFERENT from the `.goreleaser.yaml` top-level `version: 2` (the SCHEMA version). Don't conflate them.

# GOTCHA — S1 runs in parallel and creates .github/workflows/ci.yml + .golangci.yml. S2's release.yml
# is a SEPARATE file (no edit conflict). Both live in .github/workflows/. Ensure release.yml's
# `name:` is distinct ("release") and its `on:` trigger is tags-only (no push(main)/pull_request).
```

#### Namespace decision tree (resolves the #1 gotcha — read before tagging)

```
CONFIRM the intended canonical repo:
  git remote -v        # currently: dabstractor/stagecoach
  head -1 go.mod       # module github.com/dustin/stagecoach  <- source of truth for `go install`

The config uses `dustin/stagecoach` + `dustin/homebrew-tap` + `dustin/scoop-bucket` (PRD contract +
go.mod module path). `goreleaser check` + snapshot are UNAFFECTED by the real remote (publish nothing).

BEFORE THE FIRST REAL RELEASE (git tag + push), the human MUST ensure one of:
  (A) PREFERRED: the repo is reachable at github.com/dustin/stagecoach
      (rename/transfer dabstractor/stagecoach → dustin/stagecoach, or create dustin/stagecoach mirroring
       the module path). Then `go install github.com/dustin/stagecoach/cmd/stagecoach@latest` works (PRD
       §21.3) and goreleaser publishes the GitHub Release to dustin/stagecoach with GITHUB_TOKEN.
  (B) If the project permanently adopts `dabstractor`: that is a SEPARATE change touching go.mod module
      path + PRD §21.3 install commands + every owner ref in this config + the tap/bucket owners. Out
      of S2's scope — flag it; do NOT make that decision here.

Either way, S2 SHIPS THE CONFIG AS-IS (dustin namespace) and validates via snapshot. Recording the
chosen owner + the open (A)/(B) question in the commit message / PR description is REQUIRED.
```

#### formats-vs-format decision (resolves the archives uncertainty)

```
USE `formats: [tar.gz]` + `format_overrides: [{goos: windows, formats: [zip]}]` (plural, current v2).

RUN `goreleaser check`:
  - if exit 0  -> keep plural `formats`. DONE.
  - if it errors on `formats` (older installed minor) -> switch to singular:
        format: tar.gz
        format_overrides:
          - goos: windows
            format: zip
    re-run `goreleaser check` -> expect exit 0.
Do NOT mix plural and singular in the same file.
```

#### AUR decision (resolves the #4 gotcha)

```
INCLUDE the native `aurs:` block for `stagecoach-bin` (prebuilt) — best-effort, over SSH.

RUN `goreleaser check` with the `aurs:` block present:
  - if exit 0  -> keep it. (The real release needs AUR_SSH_PRIVATE_KEY + an AUR account; snapshot
                 publishes nothing, so check is the gate.)
  - if it errors on a field name (private_key / provides / conflicts / etc.) -> consult
    https://goreleaser.com/customization/aur/ ; if a quick fix isn't obvious, COMMENT OUT the entire
    `aurs:` block and ship brew+scoop+checksum+changelog+archives (the core contract). Document the
    AUR deferral in the commit message. AUR does NOT block the rest.

From-source `stagecoach` AUR package is OUT of scope (PRD "possibly community"; native pipe can't emit
a go-build PKGBUILD). If desired later: hand-author a PKGBUILD + an after.hooks submit step (future task).
```

## Implementation Blueprint

### Implementation Tasks (ordered by dependencies)

```yaml
Task 0: VERIFY prerequisites (READ + RUN, no edit)
  - RUN: `git remote -v` -> note owner (currently `dabstractor`). READ the "Namespace decision tree".
  - RUN: `head -1 go.mod` -> `module github.com/dustin/stagecoach` (source of truth for `go install`).
  - RUN: `grep -n 'var version' cmd/stagecoach/main.go` -> confirms `var version = "dev"` (ldflags target).
  - RUN: `grep -n 'main.version\|MAIN_PKG' Makefile` -> confirms symbol `main.version`, pkg `./cmd/stagecoach`.
  - RUN: `grep -n '/dist/' .gitignore` -> confirms `dist/` ignored (no .gitignore edit needed).
  - RUN: `which goreleaser` -> expect NOT FOUND; install per Task 1.
  - CONFIRM `.goreleaser.yaml` + `.github/workflows/release.yml` do NOT exist (`ls .goreleaser.yaml
    .github/workflows/release.yml` -> no such file). Created by writing them.

Task 1: INSTALL goreleaser v2 (validation prerequisite — CRITICAL #3)
  - RUN: `go install github.com/goreleaser/goreleaser/v2@latest`   # NOTE the /v2 module path
    (fallback: `brew install goreleaser`, or see https://goreleaser.com/install/)
  - RUN: `goreleaser --version` -> confirm v2.x. If `go install …/v2@latest` fails (GOPATH/bin not on
    PATH), ensure `$(go env GOPATH)/bin` is on $PATH.
  - NOTE: this installs a dev tool, not a project file; it does not touch go.mod (it's outside the module).

Task 2: CREATE .goreleaser.yaml (the config)
  - CONTENT: copy §".goreleaser.yaml" below verbatim, then run the two DECISION gates:
      (a) "formats-vs-format decision" — keep plural `formats` unless `goreleaser check` rejects it.
      (b) "AUR decision" — keep `aurs:` unless `goreleaser check` rejects a field and docs don't resolve.
  - VERIFY: `goreleaser check` -> exit 0. Iterate on any reported key/schema error using the cited docs.
  - KEY INVARIANTS: `version: 2`; builds[0].env has `CGO_ENABLED=0`; goos=[linux,darwin,windows];
    goarch=[amd64,arm64]; ldflags has `-X main.version={{.Version}}`; archives name_template uses
    `stagecoach_{{.Version}}_{{.Os}}_{{.Arch}}`; windows -> zip.

Task 3: CREATE .github/workflows/release.yml (the tag-triggered workflow)
  - CONTENT: copy §".github/workflows/release.yml" below verbatim.
  - INVARIANTS: `on: push: tags: ['v*']` ONLY; `actions/checkout@v4` with `fetch-depth: 0`;
    `actions/setup-go@v5` with `go-version-file: go.mod`; `goreleaser/goreleaser-action@v6` with
    `args: release --clean`; env maps GITHUB_TOKEN + the 3 PAT/SSH secrets.
  - VERIFY: `actionlint .github/workflows/release.yml` -> exit 0 (fallback: python YAML parse).

Task 4: VALIDATE end-to-end with snapshot (publishes NOTHING — the contract's "Mock")
  - RUN: `goreleaser release --snapshot --clean` -> expect success, no publishing.
  - INSPECT dist/: `ls dist/` -> exactly 6 archives (`stagecoach_*_{linux,darwin,windows}_{amd64,arm64}`
    with windows=zip, others=tar.gz) + `stagecoach_*_checksums.txt` + a changelog + 6 binaries.
  - PROVE ldflags: pick one binary, e.g.
      `./dist/stagecoach_linux_amd64_v1/stagecoach --version`
      (or `goreleaser build --single-target --snapshot --clean` then run the built binary) ->
      expect a SYNTHETIC version (0.0.0-next / 0.0.0-SNAPSHOT-…), NOT `dev`. (Synthetic is correct;
      the point is the -X injection fired.)
  - PROVE checksums cover all 6: `cat dist/stagecoach_*_checksums.txt | wc -l` -> 6 lines (one per archive).
  - PROVE windows archives are zip: `ls dist/*windows*.zip` -> 2 files; `ls dist/*windows*.tar.gz` -> none.

Task 5: SCOPE & CLEANLINESS checks
  - RUN: `git status --short` -> ONLY `.goreleaser.yaml` + `.github/workflows/release.yml`.
  - RUN: `grep -nE 'push:|pull_request:|branches:' .github/workflows/release.yml` -> ONLY `tags: ['v*']`
    (NO push(main), NO pull_request — those are ci.yml / S1).
  - RUN: `git diff --stat` against the base -> confirm Makefile, main.go, go.mod, ci.yml, .golangci.yml
    UNCHANGED.
  - RECORD in the commit/PR: the chosen owner namespace (`dustin`) + the open (A)/(B) namespace
    question (so the human resolves it before the first real tag), and whether AUR was kept or deferred.
```

### Implementation Patterns & Key Details

#### `.goreleaser.yaml` (copy-pasteable; run the 2 decision gates after pasting)

```yaml
# .goreleaser.yaml — Stagecoach release config (goreleaser v2). PRD §21.2.
# Docs: https://goreleaser.com/customization/
# Validate: `goreleaser check`  then  `goreleaser release --snapshot --clean` (publishes NOTHING).
#
# Owner note: uses `dustin/stagecoach` (+ dustin/homebrew-tap, dustin/scoop-bucket) per PRD §21.2/§21.3
# and the go.mod module path (github.com/dustin/stagecoach). See PRP "Namespace decision tree": the
# current git remote is `dabstractor/stagecoach`; before the first REAL tag the repo must be reachable
# at github.com/dustin/stagecoach (or the namespace is reconciled repo-wide). Snapshot validation is
# unaffected (it publishes nothing).
version: 2

project_name: stagecoach

# Optional hygiene. Safe for a clean module; remove if it ever errors offline.
before:
  hooks:
    - go mod tidy

builds:
  - id: stagecoach
    main: ./cmd/stagecoach          # matches Makefile MAIN_PKG; cmd/stagecoach/main.go has `var version`
    binary: stagecoach              # name INSIDE the archive
    env:
      - CGO_ENABLED=0              # REQUIRED: static cross-compile (no C cross-compiler on CI). CRITICAL #2.
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    # -> exactly 6 targets: linux/darwin/windows × amd64/arm64 (PRD §21.2/G9). No 32-bit ARM/MIPS -> no goarm/gomips.
    ldflags:
      - -s -w -X main.version={{.Version}}   # PRD §21.1. {{.Version}} = tag without leading "v" (e.g. 1.0.0).

archives:
  - id: default
    ids:
      - stagecoach                  # references builds[0].id
    name_template: >-
      {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}
    # e.g. stagecoach_1.0.0_linux_amd64.tar.gz / stagecoach_1.0.0_windows_amd64.zip
    formats:
      - tar.gz                     # DECISION GATE: if `goreleaser check` rejects `formats`, use `format: tar.gz`.
    format_overrides:
      - goos: windows
        formats:
          - zip                    # windows -> zip; everything else -> tar.gz.

checksum:
  name_template: '{{ .ProjectName }}_{{ .Version }}_checksums.txt'
  algorithm: sha256

changelog:
  sort: asc
  filters:
    exclude:
      - '^docs:'
      - '^test:'
      - '^chore:'
      - '^ci:'
      - '^release:'
      - 'Merge pull request'
      - 'Merge branch'

release:
  github:
    owner: dustin                  # explicit; WINS over git-remote auto-detect (which is `dabstractor`).
    name: stagecoach
  draft: false
  prerelease: auto                 # prerelease if the tag looks like a pre-release (e.g. -rc1).

brews:
  - name: stagecoach
    ids:
      - default                    # archive id this formula pulls binaries from
    repository:
      owner: dustin
      name: homebrew-tap           # PRD §21.2: tap repo github.com/dustin/homebrew-tap
      token: '{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}'   # fine-grained PAT, contents:write on the tap repo
    homepage: https://github.com/dustin/stagecoach
    description: 'Snapshot-based AI commit message generator that uses YOUR local CLI agent'
    license: MIT                   # ADJUST to the repo's actual license if different.
    install: |
      bin.install "stagecoach"
    test: |
      system "#{bin}/stagecoach", "--version"
    commit_author:
      name: goreleaserbot
      email: bot@goreleaser.com

scoops:
  - name: stagecoach
    ids:
      - default
    repository:
      owner: dustin
      name: scoop-bucket           # PRD §21.3: scoop install dustin/stagecoach -> bucket github.com/dustin/scoop-bucket
      token: '{{ .Env.SCOOP_BUCKET_GITHUB_TOKEN }}'
    homepage: https://github.com/dustin/stagecoach
    description: 'Snapshot-based AI commit message generator that uses YOUR local CLI agent'
    license: MIT                   # ADJUST to the repo's actual license if different.
    url_template: 'https://github.com/dustin/stagecoach/releases/download/{{ .Tag }}/{{ .ArtifactName }}'

# AUR — BEST-EFFORT (PRD §21.2; contract LOGIC bullets don't list AUR, so this is nice-to-have).
# Native pipe produces stagecoach-BIN only (prebuilt binary PKGBUILD), pushed over SSH (NOT a PAT).
# From-source `stagecoach` AUR package is OUT of scope (manual/community PKGBUILD; PRD "possibly community").
# DECISION GATE: if `goreleaser check` rejects any field below, COMMENT OUT this whole `aurs:` block and
# ship the rest (brew+scoop+checksum+changelog+archives). AUR does not block the core contract.
aurs:
  - name: stagecoach-bin
    ids:
      - default
    homepage: https://github.com/dustin/stagecoach
    description: 'Snapshot-based AI commit message generator that uses YOUR local CLI agent'
    maintainers:
      - 'Dustin <dustin@example.com>'   # REPLACE with the maintainer's real name <email>.
    license: MIT                        # ADJUST to the repo's actual license if different.
    depends:
      - git
    provides:
      - stagecoach
    conflicts:
      - stagecoach                       # conflicts with a hypothetical from-source `stagecoach` package
    private_key: '{{ .Env.AUR_SSH_PRIVATE_KEY }}'   # SSH private key whose pubkey is on the AUR account
    commit_author:
      name: goreleaserbot
      email: bot@goreleaser.com
```

#### `.github/workflows/release.yml` (copy-pasteable)

```yaml
# .github/workflows/release.yml — Release on tag via goreleaser (PRD §20.4, §21.2).
# Triggered ONLY on `v*` tags. CI (build/test/lint/coverage) lives in ci.yml (P1.M5.T3.S1).
# Required repo SECRETS (Settings → Secrets → Actions), needed ONLY for a REAL release (not for config
# validation; `goreleaser release --snapshot` publishes nothing):
#   HOMEBREW_TAP_GITHUB_TOKEN  fine-grained PAT, contents:write on dustin/homebrew-tap
#   SCOOP_BUCKET_GITHUB_TOKEN  fine-grained PAT, contents:write on dustin/scoop-bucket
#   AUR_SSH_PRIVATE_KEY        SSH private key for the AUR account (only if the `aurs:` block is kept)
# (GITHUB_TOKEN is auto-provided — no secret needed for the GitHub Release itself.)
name: release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write   # create the GitHub Release + upload assets

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0   # MANDATORY: goreleaser needs full history for the changelog.

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod   # uses the `go` directive in go.mod (1.22); keeps CI == module.
          cache: true

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          version: latest           # the goreleaser BINARY version (pin e.g. ~> v2 for reproducibility).
          args: release --clean     # --clean removes dist/ first (replaces removed --rm-dist).
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}
          SCOOP_BUCKET_GITHUB_TOKEN: ${{ secrets.SCOOP_BUCKET_GITHUB_TOKEN }}
          AUR_SSH_PRIVATE_KEY: ${{ secrets.AUR_SSH_PRIVATE_KEY }}
```

### Integration Points

```yaml
GITHUB ACTIONS (new, alongside S1's ci.yml):
  - release.yml lives at .github/workflows/release.yml (auto-discovered). Triggers ONLY on tag 'v*'.
    ci.yml (S1) triggers on push(main)+PR. The two never collide (distinct triggers + distinct files).
  - REQUIRED SECRETS (Settings → Secrets): HOMEBREW_TAP_GITHUB_TOKEN, SCOOP_BUCKET_GITHUB_TOKEN,
    AUR_SSH_PRIVATE_KEY (only if `aurs:` kept). GITHUB_TOKEN is auto-provided. These are a release-time
    setup step; their absence does NOT affect `goreleaser check` or snapshot validation.

MAKEFILE (UNCHANGED — independent):
  - goreleaser runs its own `builds:` (with its own ldflags/env); it does NOT call `make build`. The
    Makefile's `VERSION`/`LDFLAGS` stay for local dev. Both set `main.version` (agreement by
    construction). `make clean` already removes `dist/` — consistent with `goreleaser --clean`.

MAIN.GO (UNCHANGED — already wired):
  - `var version = "dev"` + `cmd.Version = version` already exist. goreleaser's `-X main.version=…`
    sets the symbol at build time; cobra's `--version` prints it. No code change.

GO.MOD (UNCHANGED):
  - `module github.com/dustin/stagecoach` is the `go install` identity (PRD §21.3) and goreleaser's
    `.ModulePath`. `go-version-file: go.mod` in release.yml pins Go to the module's `go 1.22`.

SCOPE HANDOFFS (do NOT create/edit — owned elsewhere):
  - S1 (P1.M5.T3.S1): .github/workflows/ci.yml + .golangci.yml. release.yml is tags-only; no overlap.
  - S3 (P1.M5.T3.S3): a `make coverage-gate` Makefile target. S2 does not touch the Makefile.
  - P1.M5.T4: README + the §21.3 install commands + the install.sh curl|sh one-liner. OUT of S2 scope.
  - From-source `stagecoach` AUR: manual/community PKGBUILD (future), not goreleaser. OUT of scope.

PARALLEL (P1.M5.T3.S1, in-flight): adds ci.yml + .golangci.yml. release.yml is a separate file — no
  conflict. (If S1 also adds a `.github/workflows/` dir, both files coexist; order is irrelevant.)
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# CRITICAL: goreleaser is NOT installed by default — install v2 first (CRITICAL #3):
go install github.com/goreleaser/goreleaser/v2@latest    # NOTE the /v2 module path
# ensure it's on PATH:  export PATH="$PATH:$(go env GOPATH)/bin"
goreleaser --version          # expect: a v2.x line

# Validate the goreleaser config schema (catches bad keys, plural/singular mistakes, removed fields):
goreleaser check              # expect: exit 0, "configuration valid" (or similar)
# If it errors on `formats` -> apply the "formats-vs-format decision" (switch to singular format:).
# If it errors on an `aurs:` field -> apply the "AUR decision" (comment out the aurs: block).

# Validate the workflow YAML (best offline check):
go install github.com/rhysd/actionlint/cmd/actionlint@latest
actionlint .github/workflows/release.yml     # expect: exit 0
# Fallback if actionlint unavailable:
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml')); print('ok')"
python3 -c "import yaml; yaml.safe_load(open('.goreleaser.yaml')); print('ok')"

# (No Go source files are added/changed by S2, so gofmt/vet are no-ops here.)
# Expected: zero errors from goreleaser check + actionlint.
```

### Level 2: Snapshot build (Component Validation — the contract's "Mock")

```bash
# Build ALL artifacts locally WITHOUT publishing (snapshot publishes NOTHING — guaranteed):
goreleaser release --snapshot --clean

# Inspect dist/ — expect exactly 6 archives + checksums + changelog + 6 binaries:
ls dist/
ls dist/*.tar.gz dist/*.zip                        # 4 tar.gz (linux/darwin ×2 arch) + 2 zip (windows ×2 arch) = 6
ls dist/*windows*.zip && ! ls dist/*windows*.tar.gz # windows archives are zip, NOT tar.gz
cat dist/*_checksums.txt | wc -l                    # 6 lines — one checksum per archive

# Prove ldflags injection fired (version is NOT "dev"):
# (snapshot uses a synthetic version like 0.0.0-next / 0.0.0-SNAPSHOT-<sha> — synthetic is CORRECT)
./dist/stagecoach_linux_amd64_v1/stagecoach --version   # expect: "stagecoach version 0.0.0…" NOT "dev"
# (path suffix _v1 is goreleaser's build-id dir; adjust to what ls showed. On non-linux hosts use the
#  matching binary, or: goreleaser build --single-target --snapshot --clean  then run that binary.)
```

### Level 3: Integration (System Validation — real-release path; needs secrets + a fetchable repo)

```bash
# A REAL release requires (release-time setup, NOT needed for snapshot):
#   1. repo reachable at github.com/dustin/stagecoach  (see "Namespace decision tree")
#   2. secrets HOMEBREW_TAP_GITHUB_TOKEN, SCOOP_BUCKET_GITHUB_TOKEN, (AUR_SSH_PRIVATE_KEY if aurs kept)
#   3. target repos exist (dustin/homebrew-tap, dustin/scoop-bucket) with the PAT granted contents:write
#   4. an AUR account + registered SSH key (if aurs kept)

# Cut a dry-run release (PRERELEASE so users aren't affected):
git tag -a v0.0.0-rc1 -m "release dry-run"
git push origin v0.0.0-rc1
# Watch the Actions tab: the `release` workflow runs, goreleaser builds the 6 targets, creates a
# GitHub Release (prerelease), uploads archives+checksums+changelog, and pushes the brew/scoop/aur.
# Verify on GitHub: Releases tab shows 6 archives + checksums.txt + changelog. Verify:
#   brew tap dustin/homebrew-tap && brew install stagecoach && stagecoach --version
#   (Scoop/AUR analogously on Windows/Arch.)
# Clean up: delete the tag + the prerelease when done.
# Expected: full artifact set produced + all channels updated; `stagecoach --version` prints v0.0.0-rc1.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Prove the 6-combo matrix is exactly right (no missing/extra targets):
# After `goreleaser release --snapshot --clean`, the archives should enumerate:
for combo in linux_amd64 linux_arm64 darwin_amd64 darwin_arm64 windows_amd64 windows_arm64; do
  ls dist/*_${combo}.tar.gz dist/*_${combo}.zip 2>/dev/null
done   # exactly one archive per combo (6 total).

# Prove static binaries (CGO off) — should say "statically linked" or have no dynamic deps:
file dist/stagecoach_linux_amd64_v1/stagecoach    # expect: "statically linked" (CGO_ENABLED=0)
ldd dist/stagecoach_linux_amd64_v1/stagecoach 2>&1 | grep -q "not a dynamic executable" && echo "STATIC OK"

# Confirm scope discipline (no boundary violations):
git status --short                            # expect ONLY: .goreleaser.yaml + .github/workflows/release.yml
grep -nE 'push:|pull_request:|branches:' .github/workflows/release.yml   # expect ONLY tags: ['v*']
grep -nE 'on tag|tags:' .github/workflows/ci.yml 2>/dev/null && echo "WARNING: ci.yml has tags (S1 should not)" || echo "ci.yml clean (no tags) — OK"
# Confirm no edits to the frozen files:
git diff --stat -- Makefile cmd/stagecoach/main.go go.mod go.sum .golangci.yml  # expect: empty (no changes)
```

## Final Validation Checklist

### Technical Validation

- [ ] `goreleaser check` exits 0 (config schema valid; `formats`/`aurs` decision gates applied).
- [ ] `goreleaser release --snapshot --clean` succeeds and publishes NOTHING.
- [ ] `dist/` holds exactly 6 archives (linux/darwin/windows × amd64/arm64; windows=zip, others=tar.gz).
- [ ] `dist/*_checksums.txt` has 6 lines (one per archive); `algorithm: sha256`.
- [ ] A built binary's `--version` prints the injected (synthetic, not `dev`) version → ldflags fired.
- [ ] Binaries are statically linked (`CGO_ENABLED=0`).
- [ ] `actionlint .github/workflows/release.yml` exits 0 (or YAML parses).

### Feature Validation

- [ ] All success-criteria bullets under "What" met.
- [ ] `.goreleaser.yaml`: `version: 2`; `builds[0]` has `CGO_ENABLED=0`, the 3 goos + 2 goarch, and
      `-X main.version={{.Version}}` in ldflags.
- [ ] `archives` produces `stagecoach_<v>_<os>_<arch>.<ext>` (windows → zip).
- [ ] `brews` targets `dustin/homebrew-tap`; `scoops` targets `dustin/scoop-bucket`; both use a PAT env.
- [ ] `release.yml` triggers ONLY on `push: tags: ['v*']`; uses `fetch-depth: 0`; `goreleaser-action@v6`.
- [ ] AUR: either the `aurs:` block passes `goreleaser check` (kept) OR is commented out with a
      documented deferral (fallback). Either outcome satisfies the contract (AUR is best-effort).

### Code Quality & Scope Validation

- [ ] `git status --short` shows ONLY `.goreleaser.yaml` + `.github/workflows/release.yml`.
- [ ] Makefile, cmd/stagecoach/main.go, go.mod, go.sum, .golangci.yml, ci.yml UNCHANGED.
- [ ] release.yml has NO `push: branches:` / `pull_request:` trigger (those are ci.yml / S1).
- [ ] No `--rm-dist` (use `--clean`); no `replacements`/`rlcp`; no `universal_binaries`; no singular
      `brew:`/`scoop:`/`tap:`/`bucket:` (v2 uses plural + `repository:`).
- [ ] `permissions: contents: write`; action versions pinned (checkout@v4, setup-go@v5, goreleaser-action@v6).

### Documentation & Deployment

- [ ] `.goreleaser.yaml` header comment documents: v2, the validate commands, the namespace note.
- [ ] `release.yml` header comment documents: tags-only trigger, required secrets, that snapshot needs none.
- [ ] The commit/PR records the chosen owner namespace (`dustin`) + the open (A)/(B) namespace question
      (human resolves before the first real tag) + whether AUR was kept or deferred.

---

## Anti-Patterns to Avoid

- ❌ Don't omit `CGO_ENABLED=0` — the linux/darwin/windows × arm64 cross-builds fail without it.
- ❌ Don't add a `before:` hook that runs `make build` (or rely on the Makefile's VERSION) — goreleaser
      runs its own `builds:` with its own ldflags; they're independent. Both set `main.version` by design.
- ❌ Don't use `--rm-dist`, `replacements`/`rlcp`, `universal_binaries`, or singular `brew:`/`scoop:`/
      `tap:`/`bucket:` — all removed/renamed in v2. Use `--clean`, `name_template`, and plural keys + `repository:`.
- ❌ Don't set `fetch-depth` to 1 (default) — the changelog is truncated. Use `fetch-depth: 0`.
- ❌ Don't expect `${{ secrets.GITHUB_TOKEN }}` to push to the tap/bucket/AUR — it's scoped to the
      workflow's repo. Use separate PATs (brew/scoop) + an SSH key (AUR).
- ❌ Don't conflate the goreleaser-action `version:` input (the binary) with the `.goreleaser.yaml`
      top-level `version: 2` (the schema). Both must be v2-line.
- ❌ Don't silently switch the owner to `dabstractor` to match the current remote — that contradicts the
      go.mod module path and breaks `go install`. Keep `dustin`; flag the namespace for human resolution.
- ❌ Don't treat AUR as a hard requirement — the contract LOGIC bullets don't list it. If the native
      `aurs:` pipe fails `goreleaser check`, comment it out and ship the core (brew/scoop/checksum/
      changelog/archives). AUR doesn't block the rest.
- ❌ Don't ship a config you haven't run `goreleaser check` + `goreleaser release --snapshot --clean`
      against — install goreleaser (it's not present by default) and validate before committing.
- ❌ Don't add a tags trigger or a release step to ci.yml — release.yml owns tag/release (S2's scope).
```
