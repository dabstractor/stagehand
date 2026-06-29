# Go Module Scaffold — Empirical Probe (go1.26.4)

> Verified by execution in a throwaway temp module on 2026-06-29 against the
> installed toolchain `go version go1.26.4-X:nodwarf5 linux/amd64`.

## 1. `go mod init` has NO `-go` flag (CRITICAL)

```
$ go mod init -go=1.22 github.com/dustin/stagehand
flag provided but not defined: -go
usage: go mod init [module-path]
Run 'go help mod init' for details.
EXIT=2
```

**Correct two-step sequence** (this is the canonical way; do NOT try to pass `-go`):

```
$ go mod init github.com/dustin/stagehand   # writes: go 1.26.4
$ go mod edit -go=1.22                       # rewrites: go 1.22  (no toolchain directive)
```

Resulting minimal `go.mod` (exactly as the contract requires — "minimal go.mod (go 1.22)"):

```
module github.com/dustin/stagehand

go 1.22
```

- `go mod edit -go=1.22` does **not** inject a `toolchain` directive. Good — keeps it minimal.
- A `go 1.22` directive under the 1.26.4 toolchain does NOT trigger a toolchain download
  (current >= required), so `go build ./...` runs on the installed toolchain without network.

## 2. Empty package directories are SILENTLY SKIPPED by the Go toolchain

Created `internal/git`, `internal/config`, `internal/provider`, `pkg/stagehand`, `providers`,
`docs` as truly empty directories, plus a `cmd/stagehand/main.go` stub:

```
$ go build ./...   # EXIT=0
$ go vet ./...     # EXIT=0
$ go list ./...    # → github.com/dustin/stagehand/cmd/stagehand   (only the package with a .go file)
```

**Implication:** the contract instruction "Create empty .go files only where needed to make
`go build ./...` pass (a main.go stub)" is exactly right. **Only `cmd/stagehand/main.go` is
required.** Empty `internal/*` and `pkg/stagehand` dirs produce zero build errors.

- Caveat: Git does not track empty directories. This is acceptable because every package
  directory receives real `.go` source files in its own dedicated later subtask (T2=git,
  T3=diff/log/stage, T4=config, etc.). `go build ./...` does not depend on their existence.

## 3. The minimal main.go stub

```go
package main

func main() {}
```

- `gofmt -l cmd/stagehand/main.go` → empty (already canonical formatting). No reformatting needed.
- Stub is intentionally a no-op; arg parsing / wiring arrive in P1.M4 (CLI layer).

## 4. .gitignore delta vs. the existing repo-root .gitignore

The repo already has a `.gitignore`. The contract requires four entries: `/bin/`, `*.test`,
`coverage.out`, `dist/`. Status against the current file:

| Required entry | Present? | Note |
|---|---|---|
| `/dist/` (== contract `dist/`) | ✅ already present | anchored at root |
| `/bin/` | ❌ MISSING | **must add** — §21.1 builds to `./bin/stagehand` |
| `*.test` | ❌ MISSING | **must add** — Go test binaries from `go test -c` |
| `coverage.out` | ❌ MISSING | **must add** — `go test -cover` output (§20.3) |

Action: PRESERVE all existing entries (build artifacts, /node_modules/, /venv/, .env*, OS files,
editor swap files) and APPEND the three missing entries. Do NOT ignore `plan/`, `PRD.md`, or any
task files (per forbidden operations).
