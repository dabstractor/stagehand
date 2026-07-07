# Research: GoReleaser v2 Configuration for Go CLI Distribution (`stagecoach`)

> **CRITICAL TOOLING CAVEAT — READ FIRST.**
> This session had **only `read` and `write` tools available**; there was **no `web_search` tool**, so **no URL was live-fetched or verified in this run**. Every URL below is reconstructed from knowledge of the stable `goreleaser.com` doc-site structure and is marked `[NOT LIVE-VERIFIED]`. Before relying on any field name — **especially the AUR pipe (§6) and the `formats` vs `format` archives change (§3)** — open the cited page and confirm against the **current** docs (GoReleaser ships frequently and has renamed keys across v2.x patches). Confidence markers: **[HIGH]** stable/long-standing, **[MED]** verify field name, **[LOW]** verify existence + semantics.
>
> The configs in §13–§14 are written to be correct-and-minimal but must be validated with `goreleaser check` (§10) before commit.

---

## Summary

GoReleaser v2 uses **plural section keys** (`brews:`, `scoops:`, `aurs:`) and a **`repository:` sub-object** (with `owner`/`name`/`token`) that **replaces the v1 `tap:`/`bucket:` keys**. It requires a top-level **`version: 2`**. Cross-compiling the 6 linux/darwin/windows × amd64/arm64 targets requires `env: [CGO_ENABLED=0]` in `builds:`; archives split windows→`zip` via `format_overrides`; checksums and a sorted changelog are built-in. There **is a native `aurs:` pipe** in v2 (key is `aurs:`, plural) that emits `-bin`-style PKGBUILDs and pushes over **SSH** (not a PAT) — but it does **not** generate a from-source `stagecoach` package; for that you hand-maintain a PKGBUILD. The canonical CI flow is `on: push: tags: ['v*']` → `checkout@v4 (fetch-depth: 0)` → `setup-go@v5` → `goreleaser-action@v6 (args: release --clean)`, with a **separate PAT** secret per external repo (tap/bucket) and an **SSH key** for AUR.

---

## 1. Top-level: `version: 2` and YAML structure  **[HIGH]**

- **`version: 2` is required** at the top of `.goreleaser.yaml` when running the v2 binary. Running a v2 binary against a v1 config (no `version:` or `version: 1`) produces an error/directs you to migrate.
- Top-level keys you'll touch: `version`, `project_name`, `dist` (default `dist/`), `before`, `builds`, `archives`, `checksum`, `snapshot`, `changelog`, `release`, `brews`, `scoops`, `aurs`, `gomod`, `source` (source-archive), `metadata`.
- Customization index: `https://goreleaser.com/customization/` `[NOT LIVE-VERIFIED]`. Quickstart: `https://goreleaser.com/quick-start/`. Schema/limit reference: `https://goreleaser.com/customization/#index` and `https://goreleaser.com limitations/` (the latter covers size/time caps).

---

## 2. `builds:` section  **[HIGH]**

Exact v2 keys:

```yaml
builds:
  - id: stagecoach            # unique id; referenced by archives/brews/scoops/aurs via `ids`
    main: ./cmd/stagecoach    # path to main package
    binary: stagecoach        # output binary name (inside the archive)
    env:
      - CGO_ENABLED=0        # REQUIRED for static cross-compile — see §11
    goos:   [linux, darwin, windows]
    goarch: [amd64, arm64]
    ldflags:
      - -s -w -X main.version={{.Version}}
```

- This produces **exactly 6 binaries** (3 × 2). No `goarm`/`gomips` needed (arm64 only; no 32-bit ARM/MIPS).
- `main:` may be omitted if there's a single `main` package at repo root; **explicit is safer** because stagecoach's main lives in `./cmd/stagecoach`.
- `binary:` controls the name **inside** the archive; `project_name:` controls the archive **prefix**.

**Template var semantics (build-time ldflags context):**  **[HIGH]**
- `{{.Version}}` → tag with the **leading `v` stripped**. Tag `v1.2.3` → `1.2.3`. This is what you want for `-X main.version=`.
- `{{.Tag}}` → the **full git tag** incl. `v`: `v1.2.3`.
- `{{.Date}}` → commit timestamp, RFC3339 (e.g. `2026-06-29T12:00:00Z`).
- `-s -w` strips symbol/DWARF info (smaller binary). Keep `-X main.version=…` exactly as the stagecoach `main.go` declares `var version = "dev"` (confirmed in `cmd/stagecoach/main.go`).

Docs: `https://goreleaser.com/customization/builds/` `[NOT LIVE-VERIFIED]`.

---

## 3. `archives:` section  **[HIGH]** (but watch the `format`→`formats` rename — **[MED]**)

```yaml
archives:
  - id: default
    ids: [stagecoach]            # reference the build id (field is `ids:` in v2; v1 used `builds:`)
    name_template: >-
      {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}
    format: tar.gz              # v2: PREFER `formats: [tar.gz]` (see note)
    format_overrides:
      - goos: windows
        format: zip             # v2: PREFER `formats: [zip]`
```

- `name_template` yields e.g. `stagecoach_1.2.3_linux_amd64.tar.gz` and `stagecoach_1.2.3_windows_amd64.zip`. `.Os`/`.Arch` are already lowercased here.
- **`replacements:` / `rlcp:` are NOT needed in v2.** The v1 `replacements:` map (for `darwin→Darwin` etc.) and the `rlcp` toggle were **removed/deprecated** in the v1→v2 transition. Use `name_template` with filters like `{{ title .Os }}` if you want title-casing. **Do not add `replacements:` or `rlcp:` to a v2 config** — `goreleaser check` will flag/error.
- **`format` → `formats` rename [MED, VERIFY]:** Recent v2 releases (≈ v2.4–v2.6, late 2024 / early 2025) introduced **`formats:`** (a **list**, e.g. `formats: [tar.gz, tar]`) to allow emitting **multiple** archive formats per target, and deprecated the singular `format:` / `format_overrides[].format`. The singular form **may still be accepted as a deprecated alias** in the version you install — confirm on the archives page and prefer `formats` (plural) for a fresh config.

Docs: `https://goreleaser.com/customization/archive/` `[NOT LIVE-VERIFIED]` (note: URL slug is `archive` singular even though the config key is `archives:` plural).

---

## 4. `brews:` section  **[HIGH]** (v2 plural + `repository:`)

- **v2 key is `brews:`** (plural list). The v1 `brew:` (singular) with `tap:` is **removed** in v2.
- **`repository:`** replaces v1's `tap:`. Sub-fields: `owner`, `name`, `branch` (optional, default repo default), `token`.

```yaml
brews:
  - name: stagecoach
    ids: [default]                       # archive id(s) this formula pulls from
    repository:
      owner: dustin
      name: homebrew-tap                 # tap repo: github.com/dustin/homebrew-tap
      token: '{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}'   # see §9 / token defaulting
    homepage: https://github.com/dustin/stagecoach     # NOTE: `homepage`, NOT `home`
    description: "stagecoach — …"
    license: "MIT"
    install: |
      bin.install "stagecoach"
    test: |
      system "#{bin}/stagecoach", "--version"
    caveats: "…"                          # optional string shown on install
    commit_author:
      name: goreleaserbot
      email: bot@goreleaser.com
    directory: Formula                    # optional; default places formula appropriately
```

**Corrections to the task's assumptions:**
- The field is **`homepage:`**, **not** `home:`. `[MED—VERIFY, but HIGH confidence]`
- `ids:` references the **archive** id (plural `ids:` in v2; v1 used `builds:`).
- **Token defaulting [HIGH]:** if you omit `repository.token`, the brew pipe checks `HOMEBREW_TAP_GITHUB_TOKEN` first, then falls back to `GITHUB_TOKEN`. Scoop pipe analogously checks `SCOOP_BUCKET_GITHUB_TOKEN`. Naming your CI secret `HOMEBREW_TAP_GITHUB_TOKEN` lets you omit `token:` — but **set it explicitly** for clarity.

Docs: `https://goreleaser.com/customization/homebrew/` `[NOT LIVE-VERIFIED]` (URL slug is `homebrew`; **config key is `brews:`**).

---

## 5. `scoops:` section  **[HIGH]**

- **v2 key is `scoops:`** (plural). v1 `scoop:` with `bucket:` is **removed**.
- **`repository:`** replaces v1's `bucket:` (`owner`/`name`/`token`).

```yaml
scoops:
  - name: stagecoach
    ids: [default]
    repository:
      owner: dustin
      name: scoop-bucket                 # github.com/dustin/scoop-bucket
      token: '{{ .Env.SCOOP_BUCKET_GITHUB_TOKEN }}'
    homepage: https://github.com/dustin/stagecoach
    description: "stagecoach — …"
    license: "MIT"
    url_template: "https://github.com/dustin/stagecoach/releases/download/{{ .Tag }}/{{ .ArtifactName }}"
    short_name: stagecoach
    persist: []                           # optional
```

Docs: `https://goreleaser.com/customization/scoop/` `[NOT LIVE-VERIFIED]` (slug `scoop`; **config key `scoops:`**).

---

## 6. AUR — **VERDICT (most uncertain area)**

### Verdict
- **A native AUR pipe EXISTS in GoReleaser v2 and is documented in the main customization index** (not a hidden/experimental feature). **[MED — confirm "stable, not experimental" against the live page; it has been in-tree for multiple v2 releases.]**
- **Config key is `aurs:` (plural)**, consistent with `brews:`/`scoops:`. **[MED]** (older docs/repos may show singular `aur:`; in v2 the plural is canonical — **verify**.)
- **It produces `-bin`-style PKGBUILDs only** (downloads a prebuilt release archive + checksums, installs the binary). It does **NOT** emit a from-source `go build` PKGBUILD.
- **Push is over SSH** to `aur.archlinux.org`, **not a GitHub PAT**. You supply an SSH **private key** via env/secret. **[LOW — exact field name (`private_key`? `git_ssh_command`?) and env-var name need live verification.]**

### Producing BOTH `stagecoach` and `stagecoach-bin`
- **`stagecoach-bin` (prebuilt):** native pipe handles this. One `aurs:` entry with `name: stagecoach-bin`, `ids: [default]`.
- **`stagecoach` (from-source):** the **native pipe cannot generate a from-source `go build` PKGBUILD**. AUR naming convention: `foo`/`foo-git` build from source; `foo-bin` uses prebuilt binaries. To ship a real from-source `stagecoach`, you must **hand-author a `PKGBUILD`** that clones the repo at the tagged commit and runs `go build`. This is outside goreleaser's scope.

### Recommended approach for this project
1. Use the **native `aurs:` pipe for `stagecoach-bin`** (low effort, goreleaser-managed autoupdate).
2. For **`stagecoach` (from-source)**: maintain a **community/manual PKGBUILD** in the AUR (separate git clone of `ssh://aur@aur.archlinux.org/stagecoach.git`). Update it on each release via an **`after.hooks`** step or a small release script that templates the PKGBUILD and pushes. **This is the standard fallback** when the native pipe doesn't fit.
3. If the native pipe proves unstable in your testing, **drop it entirely** and manage **both** packages with manual PKGBUILDs + `after.hooks`. Many Go CLIs do exactly this.

### Native `aurs:` config sketch (stagecoach-bin)
```yaml
aurs:
  - name: stagecoach-bin
    ids: [default]
    homepage: https://github.com/dustin/stagecoach
    description: "stagecoach — …"
    maintainers:
      - "Dustin Sallings <dustin@spy.net>"     # REPLACE with real email
    license: "MIT"
    depends: [git]
    provides: [stagecoach]
    conflicts: [stagecoach]                      # conflicts with the from-source pkg
    private_key: '{{ .Env.AUR_SSH_PRIVATE_KEY }}'   # [LOW — verify field name]
    commit_author:
      name: goreleaserbot
      email: bot@goreleaser.com
    # package: |-  ...            # optional custom package() body; defaults install the binary
    # goamd64: v1                 # only if you build multiple amd64 levels
```

**Falling back to manual PKGBUILD** (if native pipe rejected): in `.github/workflows/release.yml` add a post-goreleaser job/`after.hooks` step that:
- renders a `PKGBUILD` from a template (`pkgver={{.Version}}`, `sha256sums` from `dist/*_checksums.txt`),
- commits + pushes to `ssh://aur@aur.archlinux.org/stagecoach.git` (and `…/stagecoach-bin.git`) using the SSH key.

Docs: `https://goreleaser.com/customization/aur/` `[NOT LIVE-VERIFIED]` — **OPEN THIS PAGE FIRST** to confirm: (a) plural `aurs:`, (b) stable vs experimental label, (c) exact key for the SSH private key, (d) `package:`/`provides`/`conflicts` field names.

---

## 7. `checksum:` and `changelog:`  **[HIGH]**

```yaml
checksum:
  name_template: '{{ .ProjectName }}_{{ .Version }}_checksums.txt'
  algorithm: sha256            # default; can be sha512 etc.

changelog:
  sort: asc                    # asc | desc | semver
  use: git                     # default; alt: github / gitlab
  filters:
    exclude:
      - '^docs:'
      - '^test:'
      - '^chore:'
      - '^ci:'
      - 'Merge pull request'
      - 'Merge branch'
  groups:                      # optional: group entries by title/regex
    - title: 'Features'
      regexp: '^.*?feat(\(.+?\))??!?:.+$'
      order: 0
    - title: 'Bug fixes'
      regexp: '^.*?fix(\(.+?\))??!?:.+$'
      order: 1
    - title: Others
      order: 999
```

- `sort: asc` sorts commit log ascending; `semver` sorts by conventional-commit semver weight.
- `filters.exclude` is a **list of regexes**; matching commit subjects are dropped from the changelog.
- Defaults exist if you omit the section entirely (basic git log, sorted asc).

Docs: `https://goreleaser.com/customization/checksum/` and `https://goreleaser.com/customization/changelog/` `[NOT LIVE-VERIFIED]`.

---

## 8. `release:` section  **[HIGH]**

```yaml
release:
  github:
    owner: dustin
    name: stagecoach
  draft: false
  prerelease: auto          # auto = prerelease if tag matches a prerelease pattern (e.g. -rc1)
  disable: false            # set true to skip GitHub Release creation
  name_template: "{{.ProjectName}}_{{.Version}}"
```

- **Auto-detection:** if `release.github` is omitted, GoReleaser reads `origin`'s owner/name from git. **Recommend setting `owner`/`name` explicitly** so a forked/cloned remote (different owner) cannot accidentally publish to the wrong account.
- `prerelease: auto` inspects the tag; `prerelease: true/false` forces.
- `draft: true` creates a draft release you publish manually.
- If git remote owner differs from intended: **explicit `release.github.owner` wins** over auto-detection — set it.

Docs: `https://goreleaser.com/customization/release/` `[NOT LIVE-VERIFIED]`.

---

## 9. `.github/workflows/release.yml`  **[HIGH]** (action version **[MED—VERIFY @v6 vs newer**)

Canonical pattern:

```yaml
name: release
on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write          # needed to create the GitHub Release + upload assets

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0   # REQUIRED: goreleaser needs full history for changelog

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod   # uses the go directive in go.mod (go 1.22)
          cache: true

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v6   # [MED—verify v6 is current; pin if reproducibility matters]
        with:
          version: latest           # or pin e.g. '~> v2' for reproducibility
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}                  # for the GitHub Release itself
          HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}   # PAT: push to homebrew-tap
          SCOOP_BUCKET_GITHUB_TOKEN: ${{ secrets.SCOOP_BUCKET_GITHUB_TOKEN }}   # PAT: push to scoop-bucket
          AUR_SSH_PRIVATE_KEY: ${{ secrets.AUR_SSH_PRIVATE_KEY }}               # SSH key: push to AUR
```

**Key facts / permissions:**
- `${{ secrets.GITHUB_TOKEN }}` (auto-provided) is enough **only** for the release in `dustin/stagecoach`. It **cannot** push to other repos (`dustin/homebrew-tap`, `dustin/scoop-bucket`) because the default token is scoped to the workflow's repo. → Use **separate PATs** (fine-grained, `contents: write` on each target repo) or a GitHub App.
- `fetch-depth: 0` is **mandatory** — without it the changelog is truncated to the shallow clone.
- `--clean` removes `dist/` before building (prevents stale artifacts). `args: release --clean` is the v2 idiomatic form (older docs used `--rm-dist`, now deprecated/removed in favor of `--clean`).
- `version: latest` (action field, distinct from the `.goreleaser.yaml` `version:`) pulls the newest goreleaser binary. Pin (`~> v2`) for reproducible releases.
- **AUR is SSH, not PAT:** set up an SSH deploy key for the AUR git repo and expose the private key via a secret; goreleaser's aur pipe uses it. (See §6 caveat on exact field/env name.)

Docs:
- Action repo/README: `https://github.com/goreleaser/goreleaser-action` `[NOT LIVE-VERIFIED]`.
- CI guide: `https://goreleaser.com/ci/` and `https://goreleaser.com/ci/actions/` `[NOT LIVE-VERIFIED]`.

---

## 10. Validation  **[HIGH]**

```bash
# Install goreleaser v2 (NOTE the /v2 module path for go install):
go install github.com/goreleaser/goreleaser/v2@latest
# or: brew install goreleaser
# or: download from https://github.com/goreleaser/goreleaser/releases

goreleaser check                       # validates .goreleaser.yaml syntax/schema (errors on bad keys)
goreleaser check --config .goreleaser.yaml

goreleaser release --snapshot --clean  # builds ALL targets + archives + checksums WITHOUT publishing
```

- **`--snapshot` guarantees NO publishing**: it skips the GitHub Release, brew, scoop, and aur pipes entirely. It's the correct local-test command. (Produces a fake version like `0.0.0-SNAPSHOT-<sha>`.)
- **`--clean`** removes `dist/` first (replaces deprecated `--rm-dist`).
- `goreleaser build --single-target --snapshot --clean` builds **one** local target fast (handy in PR CI without the full matrix).
- Other useful subcommands: `goreleaser archive`, `goreleaser changelog`, `goreleaser release --skip=publish` (older flag; prefer snapshot).

Docs:
- Install: `https://goreleaser.com/install/` `[NOT LIVE-VERIFIED]`.
- CLI: `https://goreleaser.com/cmd/goreleaser_check` and `https://goreleaser.com/cmd/goreleaser_release` `[NOT LIVE-VERIFIED]`.

---

## 11. Common pitfalls  **[HIGH]**

1. **CGO must be disabled for cross-compilation.** Without `env: [CGO_ENABLED=0]`, the darwin/windows/linux cross-builds fail on the Go toolchain's inability to find a C cross-compiler. This is **the #1 cause** of broken cross-compile in CI. Set it in `builds[].env`.
2. **We want SEPARATE amd64/arm64 archives, NOT a universal/fat binary.** GoReleaser does **not** auto-create macOS universal binaries — Go has no native fat-binary output. Default behavior (listing `amd64` and `arm64` in `goarch`) yields **separate** archives per arch, which is what we want. Do **not** add any "universal"/`lipo` step. (An older `universal_binaries:` pipe existed for darwin; it's deprecated/removed — avoid.)
3. **Archive/build `id` collisions.** Every `builds[].id` must be unique; every `archives[].id` unique; `ids:` references must match real ids. Mismatches cause "no archives found" errors.
4. **`gomod.proxy`** (`gomod: { proxy: true }`) is **not deprecated** in v2 [MED—verify]; it's optional and improves reproducibility by vendoring via the proxy. Default (`go mod download`) is usually fine. Not required for stagecoach.
5. **`before:` hooks are NOT required** for version injection — `ldflags -X main.version=…` handles it directly (confirmed: `main.go` has `var version = "dev"`). Only add `before.hooks` if you generate assets (`go generate`, embed files, etc.). `go mod tidy` as a hook is optional hygiene.
6. **`--rm-dist` is gone** in v2 — use **`--clean`**. Using the old flag errors.
7. **`replacements:`/`rlcp:` removed in v2** — use `name_template` only (§3).
8. **`brew:`/`scoop:`/`tap:`/`bucket:` (singular/old) removed in v2** — use `brews:`/`scoops:` + `repository:` (§4–5).
9. **PAT vs `GITHUB_TOKEN` confusion** — the default token can't push cross-repo; use distinct PATs (§9).
10. **`fetch-depth: 0` omitted** → broken/truncated changelog.
11. **`go-version-file: go.mod`** ensures the CI Go matches the module's `go` directive; avoids drift.
12. **AUR needs SSH, not a token** (§6).

---

## 12. Template variables  **[HIGH]**

Available in `name_template`, `ldflags`, `url_template`, `commit_msg_template`, etc.:

| Variable | Meaning | Example (tag `v1.2.3`) |
|---|---|---|
| `.Version` | tag without leading `v` | `1.2.3` |
| `.Tag` | full git tag | `v1.2.3` |
| `.Major` `.Minor` `.Patch` | semver components | `1` `2` `3` |
| `.Prerelease` | prerelease suffix | `rc1` (empty if none) |
| `.Branch` | current git branch | `main` |
| `.Commit` / `.ShortCommit` / `.FullCommit` | commit sha (short / full) | `abc1234` / full sha |
| `.Date` | commit timestamp RFC3339 | `2026-06-29T12:00:00Z` |
| `.IsSnapshot` | bool: snapshot mode | `true`/`false` |
| `.Summary` | one-line commit summary | — |
| `.GitURL` | origin URL | `https://github.com/dustin/stagecoach` |
| `.ModulePath` | Go module path | `github.com/dustin/stagecoach` |
| `.ProjectName` | from `project_name` / repo | `stagecoach` |
| `.Os` `.Arch` `.Arm` `.Amd64` `.Mips` `.X86` | target GOOS/GOARCH (normalized) | `linux` `amd64` |
| `.Env` | map of env vars | `{{ .Env.MY_VAR }}` |
| `.Title` / `.Upper` / `.Lower` / `.Trim` | string filters (Go template + sprig) | `{{ title .Os }}` |

Plus sprig-style functions: `{{ incpatch .Version }}`, `{{ time "2006-01-02" }}`, `{{ replace ... }}`, etc.

Docs: `https://goreleaser.com/customization/templates/` (full variable table) `[NOT LIVE-VERIFIED]`.

---

## 13. Canonical `.goreleaser.yaml` (minimal-but-complete, v2)

> Validate with `goreleaser check`, then `goreleaser release --snapshot --clean`. Replace bracketed placeholders (`…`, email, descriptions) with real values. The `aur`/`formats` bits are marked **[VERIFY]**.

```yaml
# .goreleaser.yaml — GoReleaser v2
# Docs: https://goreleaser.com/customization/
version: 2

project_name: stagecoach

# Optional hygiene; remove if it causes issues. NOT required for version injection.
before:
  hooks:
    - go mod tidy

builds:
  - id: stagecoach
    main: ./cmd/stagecoach
    binary: stagecoach
    env:
      - CGO_ENABLED=0          # REQUIRED: static cross-compile (see §11.1)
    goos:   [linux, darwin, windows]
    goarch: [amd64, arm64]
    ldflags:
      - -s -w -X main.version={{.Version}}

archives:
  - id: default
    ids: [stagecoach]
    name_template: >-
      {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}
    formats: [tar.gz]          # [VERIFY] prefer plural `formats`; `format:` may still work as deprecated alias
    format_overrides:
      - goos: windows
        formats: [zip]         # [VERIFY] plural `formats`

checksum:
  name_template: '{{ .ProjectName }}_{{ .Version }}_checksums.txt'
  algorithm: sha256

snapshot:
  version_template: '{{ incpatch .Version }}-dev'   # [VERIFY] v2 uses version_template (v1: name_template)

changelog:
  sort: asc
  filters:
    exclude:
      - '^docs:'
      - '^test:'
      - '^chore:'
      - '^ci:'
      - 'Merge pull request'
      - 'Merge branch'

release:
  github:
    owner: dustin
    name: stagecoach
  draft: false
  prerelease: auto
  name_template: '{{.ProjectName}}_{{.Version}}'

brews:
  - name: stagecoach
    ids: [default]
    repository:
      owner: dustin
      name: homebrew-tap
      token: '{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}'
    homepage: https://github.com/dustin/stagecoach
    description: 'stagecoach — …'      # REPLACE
    license: MIT                       # REPLACE with actual license
    install: |
      bin.install "stagecoach"
    test: |
      system "#{bin}/stagecoach", "--version"
    commit_author:
      name: goreleaserbot
      email: bot@goreleaser.com

scoops:
  - name: stagecoach
    ids: [default]
    repository:
      owner: dustin
      name: scoop-bucket
      token: '{{ .Env.SCOOP_BUCKET_GITHUB_TOKEN }}'
    homepage: https://github.com/dustin/stagecoach
    description: 'stagecoach — …'      # REPLACE
    license: MIT                       # REPLACE
    url_template: 'https://github.com/dustin/stagecoach/releases/download/{{ .Tag }}/{{ .ArtifactName }}'

# AUR: native pipe produces stagecoach-BIN only. [VERIFY all field names §6]
aurs:
  - name: stagecoach-bin
    ids: [default]
    homepage: https://github.com/dustin/stagecoach
    description: 'stagecoach — …'      # REPLACE
    maintainers:
      - 'Dustin Sallings <dustin@spy.net>'   # REPLACE email
    license: MIT                       # REPLACE
    depends: [git]
    provides: [stagecoach]
    conflicts: [stagecoach]
    private_key: '{{ .Env.AUR_SSH_PRIVATE_KEY }}'   # [VERIFY] field name + push-over-SSH semantics
    commit_author:
      name: goreleaserbot
      email: bot@goreleaser.com

# NOTE: a from-source `stagecoach` AUR package is NOT produced by goreleaser.
# Maintain it manually (PKGBUILD + after.hooks or a release script). See §6 fallback.
```

---

## 14. Canonical `.github/workflows/release.yml`

```yaml
name: release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write   # create GitHub Release + upload assets

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - name: GoReleaser
        uses: goreleaser/goreleaser-action@v6   # [VERIFY] v6 current as of this writing
        with:
          # Pin for reproducibility if preferred: version: '~> v2'
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}
          SCOOP_BUCKET_GITHUB_TOKEN: ${{ secrets.SCOOP_BUCKET_GITHUB_TOKEN }}
          AUR_SSH_PRIVATE_KEY: ${{ secrets.AUR_SSH_PRIVATE_KEY }}
```

**Required secrets (create in repo Settings → Secrets):**
- `HOMEBREW_TAP_GITHUB_TOKEN` — fine-grained PAT, `contents: write` on `dustin/homebrew-tap`.
- `SCOOP_BUCKET_GITHUB_TOKEN` — fine-grained PAT, `contents: write` on `dustin/scoop-bucket`.
- `AUR_SSH_PRIVATE_KEY` — SSH private key whose public key is registered on the AUR account (for `stagecoach-bin`).
- (`GITHUB_TOKEN` is auto-injected — no secret needed.)

**Optional PR-gate job** (fast, non-publishing):
```yaml
  snapshot-check:
    runs-on: ubuntu-latest
    if: github.event_name == 'pull_request'
    steps:
      - uses: actions/checkout@v4
        with: { fetch-depth: 0 }
      - uses: actions/setup-go@v5
        with: { go-version-file: go.mod }
      - uses: goreleaser/goreleaser-action@v6
        with: { version: latest, args: release --snapshot --clean }
```

---

## 15. v1 → v2 naming differences (cheat sheet)

| Concept | v1 (removed/deprecated) | v2 (current) |
|---|---|---|
| Schema version | none / `version: 1` | **`version: 2`** |
| Homebrew | `brew:` + `tap:` | **`brews:`** + **`repository:`** |
| Scoop | `scoop:` + `bucket:` | **`scoops:`** + **`repository:`** |
| AUR | `aur:` (singular, early) | **`aurs:`** (plural) [VERIFY] |
| Archive ref | `builds:` (in archives) | **`ids:`** |
| Archive format | `format:` / `format_overrides[].format` | **`formats:`** (list) [VERIFY version] |
| OS renaming | `replacements:` + `rlcp:` | removed — use `name_template` |
| Clean dist | `--rm-dist` | **`--clean`** |
| Snapshot tmpl | `snapshot.name_template` | **`snapshot.version_template`** [VERIFY] |
| macOS universal | `universal_binaries:` | deprecated/removed — avoid |

---

## Sources

All URLs below are **[NOT LIVE-VERIFIED this session]** (no web tool available). They are reconstructed from the stable `goreleaser.com` doc-site structure and `goreleaser-action` README. **Open each before relying on field names**, prioritizing the AUR page.

- **Kept (high-value, recommended to open):**
  - Customization index — `https://goreleaser.com/customization/` — entry point to all section docs.
  - Builds — `https://goreleaser.com/customization/builds/` — `goos`/`goarch`/`ldflags`/`env`/`main`/`binary`/`id`.
  - Archives — `https://goreleaser.com/customization/archive/` — `formats` vs `format`, `format_overrides`, `name_template`, `replacements` removal.
  - Homebrew (brews) — `https://goreleaser.com/customization/homebrew/` — `brews:` + `repository:`, `homepage`, `test`, `install`, `caveats`.
  - Scoop (scoops) — `https://goreleaser.com/customization/scoop/` — `scoops:` + `repository:`.
  - **AUR (OPEN FIRST)** — `https://goreleaser.com/customization/aur/` — confirms `aurs:`, stable vs experimental, SSH-key field name, `package`/`provides`/`conflicts`.
  - Checksum — `https://goreleaser.com/customization/checksum/`.
  - Changelog — `https://goreleaser.com/customization/changelog/` — `sort`, `filters.exclude`, `groups`.
  - Release — `https://goreleaser.com/customization/release/` — `github.owner/name`, `draft`, `prerelease`.
  - Templates (variables table) — `https://goreleaser.com/customization/templates/` — `.Version`/`.Tag`/`.Date`/`.Major` etc.
  - CI guide — `https://goreleaser.com/ci/` and `https://goreleaser.com/ci/actions/`.
  - Install — `https://goreleaser.com/install/`.
  - CLI — `https://goreleaser.com/cmd/goreleaser_check`, `https://goreleaser.com/cmd/goreleaser_release`.
  - goreleaser-action — `https://github.com/goreleaser/goreleaser-action` (README) — action `@v6`, `version`/`args`.
- **Dropped:** none excluded by quality filter (no live search was performed, so there is no result list to prune). Any third-party blog/tutorial is excluded by default in favor of primary `goreleaser.com` / `goreleaser-action` sources.

---

## Gaps

**Could NOT be answered confidently this session (tooling-limited — no `web_search`):**

1. **AUR pipe specifics [HIGHEST PRIORITY]:** (a) exact config key spelling (`aurs:` plural vs `aur:`) in the **current** v2 docs; (b) whether docs label it "stable" or "experimental"; (c) the **exact field name for the SSH private key** (`private_key`? `git_ssh_command`? an env-var convention?); (d) default env-var name the pipe reads; (e) whether `package:`/`provides`/`conflicts`/`depends` are the exact current key names. → **Open `https://goreleaser.com/customization/aur/` and read top-to-bottom.**
2. **`formats` vs `format` archives change:** confirm the exact v2 version that introduced `formats` (plural) and whether `format:` is still accepted as a deprecated alias or fully removed. → Archives page.
3. **`snapshot.version_template` vs `name_template`:** confirm the v2 field name. → Snapshots page (`https://goreleaser.com/customization/snapshots/` or `/snapshot/`).
4. **`brews:`/`scoops:` field names:** confirm `homepage:` (not `home:`), and the exact `url_template` default for scoop. → respective pages.
5. **`goreleaser-action@v6` currency:** confirm v6 is the latest major (a v7 may exist). → action README releases.
6. **URL slugs:** confirm exact doc-site paths (e.g., `/customization/homebrew/` vs `/customization/brews/`). The config keys are plural but URL slugs historically stayed singular for brew/scoop.

**Suggested next steps:**
- Run (with web access): open the AUR page, the archives page, and the templates page; copy exact field names into §13.
- Locally: `go install github.com/goreleaser/goreleaser/v2@latest && goreleaser check` on the drafted `.goreleaser.yaml`; iterate until clean.
- Then `goreleaser release --snapshot --clean` and inspect `dist/` for the 6 archives + checksums (no publishing occurs).
- For from-source `stagecoach` AUR: draft a `PKGBUILD` template and decide between manual-maintain vs `after.hooks` automation.

---

## Supervisor coordination

No supervisor contact was made. No decision is blocked: the deliverable (this research brief) is complete within the available tooling. The one material limitation — inability to live-verify URLs/field names because no `web_search` tool was provided — is documented in the header and Gaps; the parent should either (a) accept this brief as a high-accuracy draft to be confirmed via `goreleaser check` + a quick doc read, or (b) re-run this task in an environment with web access. No `contact_supervisor` call is warranted.

```acceptance-report
{
  "criteriaSatisfied": [
    {
      "id": "criterion-1",
      "status": "satisfied",
      "evidence": "Scope confined to producing a single research brief answering all 12 requested GoReleaser v2 topics; no source/config files in the repo were modified. Output is a research document only, as scoped. Local project files (go.mod, cmd/stagecoach/main.go) were read only to confirm the module path and the ldflags version variable — no edits."
    },
    {
      "id": "criterion-2",
      "status": "partially-satisfied",
      "evidence": "Evidence provided: full findings doc with confidence markers per section, canonical .goreleaser.yaml (§13), canonical release.yml (§14), v1→v2 diff table (§15), cited URLs (§Sources), AUR verdict (§6), gotchas (§11). Limitation: NO live URL verification was possible (the session had only read/write tools; no web_search). Every URL is flagged [NOT LIVE-VERIFIED]; highest-uncertainty items (AUR field names, formats-vs-format, snapshot field name) are enumerated in Gaps with explicit verification steps. An independent reviewer should confirm field names against live goreleaser.com docs before relying on §13."
    }
  ],
  "changedFiles": [
    "research.md"
  ],
  "testsAddedOrUpdated": [],
  "commandsRun": [
    {
      "command": "read /home/dustin/projects/stagecoach/go.mod",
      "result": "passed",
      "summary": "Confirmed module github.com/dustin/stagecoach, go 1.22 — matches task assumptions."
    },
    {
      "command": "read /home/dustin/projects/stagecoach/cmd/stagecoach/main.go",
      "result": "passed",
      "summary": "Confirmed `var version = \"dev\"` exists at package main, validating the -X main.version ldflags approach and main: ./cmd/stagecoach path."
    },
    {
      "command": "(not available) goreleaser check / web_search",
      "result": "not-run",
      "summary": "No web_search tool and no local goreleaser binary were available in this session, so neither live doc verification nor `goreleaser check` could be executed. Documented as a gap; recommended as the immediate next step."
    }
  ],
  "validationOutput": [
    "No automated validation run (goreleaser not installed; no web tool to fetch docs). Static self-review performed: v2 plural keys (brews/scoops/aurs) + repository: sub-object consistently applied; version: 2 present; CGO_ENABLED=0 present; 6 cross-compile targets enumerated; --clean/--snapshot semantics confirmed; v1 keys (tap/bucket/format/replacements/--rm-dist) flagged as removed. Confidence markers [HIGH]/[MED]/[LOW] applied per section; all URLs marked [NOT LIVE-VERIFIED]."
  ],
  "residualRisks": [
    "No live URL verification: every cited goreleaser.com URL and several field names (AUR private_key field, formats-vs-format, snapshot.version_template, brews homepage vs home) are reconstructed from training knowledge and MUST be confirmed against current docs before committing the generated configs.",
    "AUR native pipe status (stable vs experimental) and exact config keys are the single largest uncertainty; §6 verdict should be re-confirmed by reading https://goreleaser.com/customization/aur/ directly.",
    "A from-source `stagecoach` AUR package is out of scope for the native pipe and requires a manual PKGBUILD — flagged as a follow-up decision (manual vs after.hooks automation).",
    "The task references a write path 'plan/001_f1f80943ac34/P1M5T3S2/research/goreleaser_research.md' but the authoritative runtime override path '/home/dustin/projects/stagecoach/research.md' was used instead per the explicit runtime instruction."
  ],
  "noStagedFiles": true,
  "diffSummary": "Created one new file: research.md (a structured GoReleaser v2 research brief). No repo source, config, or workflow files were created or modified — this is a research/planning deliverable only. The brief covers all 12 requested topics with v2-correct config keys, two canonical configs (.goreleaser.yaml + release.yml), a v1→v2 naming-diff table, a clear AUR verdict (native aurs: pipe exists for -bin; manual PKGBUILD for from-source), gotchas, cited URLs, and an explicit Gaps list driven by the session's lack of a web_search tool.",
  "reviewFindings": [
    "blocker (for downstream code generation, not for this research task): field names in §13 marked [VERIFY] (aurs.private_key, formats vs format, snapshot.version_template, brews homepage) are unverified — do not generate final repo configs from §13 without confirming against live goreleaser.com docs.",
    "no blockers for the research deliverable itself."
  ],
  "manualNotes": "Tooling constraint: this child session was given only read/write tools — there was no web_search, so the brief is training-knowledge-based with rigorous confidence marking rather than live-fetched. Highest-value follow-up: open https://goreleaser.com/customization/aur/ and the archives page, then run `goreleaser check` on the §13 draft. Also note the two conflicting output paths in the task: the runtime override (research.md) was honored; the task-body path (plan/.../goreleaser_research.md) was not, per the explicit override instruction."
}
```
