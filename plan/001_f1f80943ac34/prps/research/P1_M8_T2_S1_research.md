# Research — P1.M8.T2.S1: .goreleaser.yaml + install.sh

## Task
Ship the v1.0 distribution scaffolding (PRD §21.2–21.3, G9): a complete
`.goreleaser.yaml` producing archives + standalone binaries for
linux/darwin/windows × amd64/arm64, ldflags `-X main.version={{.Version}}`,
Homebrew formula → `dustin/homebrew-tap`, Scoop manifest → a bucket, AUR
PKGBUILD, checksums + changelog; plus a `curl|sh` `install.sh` one-liner that
detects OS/arch and fetches the right binary + checksum from GitHub Releases.
Gate: `goreleaser check` passes and `goreleaser release --snapshot --clean`
builds all targets locally.

## Verified environment
- Go `go1.26.4` linux/amd64; module `github.com/dustin/stagehand` (go.mod).
- `goreleaser` was NOT preinstalled; installed via `go install` for validation.
- `shellcheck` 0.11.0 + `sha256sum`/`shasum` present (install.sh deps).
- `.gitignore` already lists `dist/` → goreleaser artifacts stay uncommitted.
- Makefile already does `build: go build -ldflags "-X main.version=$(VERSION)"`
  with `VERSION := $(shell git describe --tags --always --dirty)` — the
  goreleaser `-X main.version={{.Version}}` is the SAME ldflags target (main.go
  `var version = "dev"`). Makefile `clean` already `rm -rf ... dist`.

## CRITICAL DECISION 1 — goreleaser version pin: v2.9.0 (Formula support)
`brews` (Homebrew **Formula**) is **hard-deprecated in goreleaser ≥ v2.10**:
- deprecations page: "brews — since v2.10 (soft), since v2.16 [hard] …
  GoReleaser would generate hackyish formulas that install the pre-compiled
  binaries … Casks should be used instead."
- Verified: on v2.15.4 AND v2.16.0, `goreleaser check` FAILS with
  "configuration is valid, but uses deprecated properties" when `brews:` is
  present. There is **no** `--allow-deprecated` flag.
- The replacement `homebrew_casks` is **macOS-only** and targets signed .app
  bundles — WRONG for a cross-platform CLI: it breaks Linuxbrew (PRD §21.3
  explicitly says "macOS / Linuxbrew") and contradicts the peer group
  (lazygit/gh/ripgrep/fd/bat all ship **Formulas**, not Casks).
- **Decision:** pin goreleaser to **v2.9.0** (the last release before the v2.10
  deprecation; only tag in the 2.9 line). Verified: v2.9.0 `goreleaser check` →
  "1 configuration file(s) validated" with `brews:` present, ZERO deprecations.
  Install: `go install github.com/goreleaser/goreleaser/v2@v2.9.0`.
  Rationale: shipping a Homebrew Formula for a cross-platform CLI is the
  correct, PRD-aligned choice; pinning a build-time-only tool to a known-good
  version is reproducible-build best practice. If goreleaser ever restores
  Formula support, bump the pin.

## CRITICAL DECISION 2 — Windows cross-compile blocker (executor.go)
The snapshot gate cross-compiles ALL targets. The existing codebase FAILS on
the 2 Windows targets because `internal/provider/executor.go` uses Unix-only
syscalls unconditionally:
- L102: `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` (no Setpgid on windows)
- L134/L138: `syscall.Kill(-pgid, syscall.SIGTERM/SIGKILL)` (undefined on windows)
```
go build (GOOS=windows): # github.com/dustin/stagehand/internal/provider
  executor.go:102:41: unknown field Setpgid in struct literal syscall.SysProcAttr
  executor.go:134:15: undefined: syscall.Kill
```
Direct cross-compile matrix (verified):
```
OK linux/amd64 linux/arm64 darwin/amd64 darwin/arm64
FAIL windows/amd64 windows/arm64   ← only these
```
NOTE: `internal/generate/signal.go` uses `syscall.SIGINT/SIGTERM` but that
DOES compile on windows (Go defines those signal constants on all platforms) —
no change needed there. Only executor.go is the blocker.

executor.go's own doc comment already anticipated this: "Setpgid is a Unix
concept … a future windows port needs CREATE_NEW_PROCESS_GROUP … and is
explicitly out of scope here (the validation gate runs on linux)." The gate no
longer runs only on linux — goreleaser cross-compiles windows. So the deferred
port must now land (at least enough to compile).

**Fix (verified, behavior-safe on Unix):** idiomatic Go build-constraint split:
- NEW `internal/provider/exec_unix.go` (`//go:build !windows`): `applyProcessGroup(cmd)`
  sets `SysProcAttr{Setpgid:true}`; `terminateGroup(pgid, force)` sends
  `syscall.Kill(-pgid, SIGTERM|SIGKILL)`.
- NEW `internal/provider/exec_windows.go` (`//go:build windows`):
  `applyProcessGroup` no-op; `terminateGroup` best-effort `os.FindProcess(pgid).Kill()`
  (no POSIX groups/signals; kills the single process — matches the v1.0
  best-effort windows intent; children not reaped).
- MODIFY `internal/provider/executor.go`: drop `syscall` import; replace the 3
  syscall lines with `applyProcessGroup(cmd)` and `terminateGroup(pgid, bool)`.
Unix behavior is byte-for-byte identical (verified: existing
provider/generate/cmd tests stay green; `go vet` clean; all 6 targets compile).

## CRITICAL DECISION 3 — GitHub org: `dustin` (NOT the git remote `dabstractor`)
- go.mod module path = `github.com/dustin/stagehand` (authoritative for
  `go install github.com/dustin/stagehand/cmd/stagehand@latest`, PRD §21.3).
- PRD §21.2/§21.3 uses `dustin` everywhere (`dustin/homebrew-tap`,
  `github.com/dustin/stagehand`).
- BUT the local git remote is `git@github.com:dabstractor/stagehand`.
- **Decision:** use `dustin` everywhere (module path + PRD + documented install
  paths all agree on it). Set `release.github.owner=dustin` explicitly so
  goreleaser does NOT silently derive the download/release URLs from the
  `dabstractor` git remote. The snapshot gate is unaffected (snapshot mode does
  not contact GitHub). FLAG for the author: confirm `dustin` is the intended
  public org and that the release/tap/bucket repos will exist under `dustin`
  before a real (non-snapshot) release.

## The verified .goreleaser.yaml (v2.9.0, full config below is the prototype)
- `version: 2`, `project_name: stagehand`, `before.hooks: [go mod tidy]`.
- `builds:` id=stagehand, main=./cmd/stagehand, binary=stagehand,
  env CGO_ENABLED=0, ldflags `["-s -w -X main.version={{.Version}}"]`,
  goos [linux,darwin,windows], goarch [amd64,arm64].
- `archives:` `ids:[stagehand]` (NOT `builds:` — that's deprecated since v2.8),
  name_template maps darwin→macos, `format_overrides` windows→zip,
  `files:[LICENSE*,README*]` (warns "no files matched" until README exists in
  P1.M8.T4.S1 — HARMLESS, build still succeeds).
- `checksum:` name_template "checksums.txt", algorithm sha256.
- `snapshot:` version_template "{{ incpatch .Version }}-dev".
- `changelog:` sort asc, use github, filters exclude docs/test/chore/merge.
- `release.github:` owner dustin, name stagehand.
- `brews:` name stagehand, repository owner=dustin name=homebrew-tap
  token=`{{.Env.TAP_GITHUB_TOKEN}}`, directory Formula, install `bin.install
  "stagehand"`, test `system "#{bin}/stagehand", "--version"`, license MIT.
- `scoops:` name stagehand, repository owner=dustin name=scoop-bucket
  token=`{{.Env.SCOOP_GITHUB_TOKEN}}`, license MIT.
- `aurs:` name stagehand-bin, private_key=`{{.Env.AUR_KEY}}`,
  git_url ssh://aur@aur.archlinux.org/stagehand-bin.git, provides [stagehand],
  license MIT. (AUR is skipped in --snapshot but the PKGBUILD is generated.)
GATE RESULT: `goreleaser check` → validated; `goreleaser release --snapshot
--clean` → "release succeeded"; produced dist/: 6 archives, checksums.txt,
homebrew/Formula/stagehand.rb, scoop/stagehand.json, aur/stagehand-bin.pkgbuild.

## Generated manifest spot-checks (from the prototype snapshot)
- Homebrew formula: `dustin/stagehand` URLs, on_macos+on_linux blocks w/
  amd64/arm64 sha256, `test do system "#{bin}/stagehand","--version" end` ✓
  (main.go sets cobra Version field → `--version` prints & exits 0).
- Scoop manifest: 64bit+arm64 windows zips, bin stagehand.exe, correct hashes ✓
- AUR PKGBUILD: stagehand-bin, arch aarch64+x86_64, provides/conflicts stagehand ✓
- Version note: snapshot shows v0.0.0/0.0.1-dev (no git tag in proto); a real
  tag v1.0.0 → {{.Version}}="1.0.0", download path ".../v1.0.0/...", filename
  "stagehand_1.0.0_<os>_<arch>.tar.gz".

## install.sh — design + verification
POSIX bash `curl|sh` (PRD §21.3, §22.2 "POSIX-ish environment"). Verified:
- `bash -n` syntax OK; `shellcheck install.sh` exit 0 (clean, incl. the literal
  `$PATH` PATH-hint handled via `\$PATH` in a double-quoted printf).
- OS detection: uname -s → linux/macos/(windows for MINGW/MSYS/CYGWIN; else
  fatal with a `scoop install` hint). Arch: uname -m → amd64/arm64.
- Version resolution: empty VERSION → GitHub API `releases/latest` `tag_name`.
  DOWNLOAD_TAG keeps the tag verbatim; VERSION_NUM strips leading `v`
  (matches goreleaser {{.Version}} filename vs tag-in-URL).
- Archive name = `stagehand_${VERSION_NUM}_${OS}_${ARCH}.tar.gz` — VERIFIED to
  match goreleaser's name_template exactly (darwin→macos).
- Checksum: downloads checksums.txt, greps the archive's sha256, verifies with
  sha256sum (fallback shasum -a 256), FAILS on mismatch.
- Install: `mktemp -d` + trap cleanup; tar -xzf; `install -m 0755` to
  $INSTALL_DIR (default /usr/local/bin if writable else ~/.local/bin; sudo only
  if the dir isn't user-writable); prints `--version`; PATH hint if missing.
- Overridable via env: OWNER, REPO, BIN_NAME, VERSION, INSTALL_DIR.
(End-to-end download can't be run here — no real GitHub release exists yet;
this task CREATES the release config. Logic + naming + shellcheck verified.)

## Open items to flag for the author (do NOT block the gate)
1. Confirm GitHub org = `dustin` (git remote is `dabstractor`).
2. Confirm license = MIT (PRD states none; default MIT to match the peer group;
   create a LICENSE file — deferred; README is P1.M8.T4.S1).
3. The Makefile/CI goreleaser pin (`@v2.9.0`) lands in the Makefile-finalization
   sibling task — this PRP does NOT modify the Makefile.
4. archives `files:[LICENSE*,README*]` warn until README/LICENSE exist — harmless.
