# Research: Makefile targets — empirically verified behaviors

> **Verified on:** GNU Make 4.4.1, `go version go1.26.4-X:nodwarf5 linux/amd64`, repo root `/home/dustin/projects/stagecoach`.
> All behaviors below were confirmed by direct execution in a throwaway temp module, NOT by the
> real repo (which has no `go.mod` yet — P1.M1.T1.S1 lands it in parallel).

## Environment facts (tool availability)

| Tool | Installed? | Version / Path |
|------|-----------|----------------|
| `make` | ✅ Yes | GNU Make 4.4.1 (`/usr/bin/make`) |
| `go` | ✅ Yes | go1.26.4-X:nodwarf5 linux/amd64 |
| `golangci-lint` | ❌ **MISSING** | not on `$PATH` |
| `govulncheck` | ❌ MISSING | not on `$PATH` (out of scope anyway — CI is P1.M5.T3.S1) |
| `goreleaser` | ❌ MISSING | not on `$PATH` (out of scope — P1.M5.T3.S2) |

**Implication:** The `lint` target must call `golangci-lint run` (per contract + PRD §21.1/§20.4)
even though the binary is absent from THIS dev box. The Makefile convention is to ASSUME the tool
is present; CI (P1.M5.T3.S1) installs it via `golangci/golangci-lint-action`, and local devs install
it from <https://golangci-lint.run/usage/install/>. For validation in THIS environment, use
`make -n lint` (dry-run, prints the command without executing) rather than requiring an install.

## VERIFIED: ldflags `-X main.version=dev` on a main.go WITHOUT a `version` var

This is the critical question: P1.M1.T1.S1 produces a stub `main.go` (`func main(){}`) with NO
`var version` declaration. Can the `build` target still use `-ldflags "-X main.version=$(VERSION)"`
without erroring?

**Test:** created `package main; func main(){}` + `go 1.22`, then:
```bash
go build -ldflags "-X main.version=dev" -o bin/test .
# → EXIT=0, binary produced
```
**Result: exit 0.** The Go linker **silently ignores** `-X importpath.name=value` when the named
symbol does not exist (documented behavior: `-X` only sets *existing* package-level string symbols;
a missing symbol is a no-op, never an error). It only errors if the symbol exists but is not a
`string`.

**Conclusion:** The `build` target can ship with ldflags NOW. It becomes effective (injects the real
version into the running binary) the moment a later subtask (P1.M4.T1, when `main.go` gains cobra
wiring) declares `var version = "dev"` at package scope. No conditional gating needed.

## VERIFIED: `go test -race ./...` and `go test -coverprofile` with NO test files

P1.M1.T1.S1 ships zero `*_test.go` files. Do the `test` / `coverage` targets still exit 0?

```
$ go test -race ./...
?   	ldflagstest	[no test files]
EXIT=0

$ go test -coverprofile=coverage.out ./...
	ldflagstest
EXIT=0
# coverage.out created (contains only the `mode: atomic` line + 0 statements)

$ go tool cover -func=coverage.out
ldflagstest/main.go:3:	main		0.0%
total:			(statements)	0.0%
EXIT=0
```

**Result: all exit 0.** A repo with no tests is not a test failure. The `coverage` target produces a
valid `coverage.out` (empty coverage) and `go tool cover -func` prints `0.0%` and exits 0.
→ Both targets are safe to run immediately after scaffolding. Real test coverage ramps up in M1.T2+.

## VERIFIED: `go install` target destination

```
$ go install .
EXIT=0
# Binary landed at /home/dustin/go/bin/ldflagstest  (i.e. $GOPATH/bin)
```

`go env GOBIN` is empty → installs fall back to `$GOPATH/bin` = `/home/dustin/go/bin`.
The `install` target (`go install ./cmd/stagecoach`) will place the binary at `~/go/bin/stagecoach`.
No special handling required; this is standard Go behavior.

## The Makefile TABS gotcha (the #1 implementation failure mode)

Makefile recipe lines MUST begin with a **TAB character**, not spaces. This is the single most
common Makefile authoring error. If the implementer copies the recipe with leading spaces:

```
Makefile:5: *** missing separator.  Stop.
```

**Mitigation in the PRP:**
- Provide the exact file content in a heredoc so tabs are unambiguous.
- Document a `cat -A Makefile | grep '^\^I'` verification (shows `^I` = tab).
- The `write` tool preserves literal tab characters embedded in the content.

## DESIGN DECISION: build target MUST include ldflags (else VERSION is dead code)

The contract says two things that only cohere if `build` uses ldflags:
1. "`build` (go build -o bin/stagecoach ./cmd/stagecoach)" — the base command.
2. "Add a `VERSION` variable defaulting to `dev` for the ldflags pattern" + PRD §21.1
   "Version injected via `-ldflags "-X main.version=…"` at release."

If `build` did NOT use ldflags, the `VERSION` variable would be unreferenced dead code. The
coherent reading: `build` = `go build -ldflags "-X main.version=$(VERSION)" -o bin/stagecoach ./cmd/stagecoach`,
preserving the exact `-o bin/stagecoach ./cmd/stagecoach` tail while adding the ldflags the VERSION
variable exists to feed. `install` mirrors this (same versioned binary).

`VERSION ?= dev` (recursive-assignment-with-default) lets release tooling override it:
`make build VERSION=v1.0.0` or goreleaser env injection → real semver (PRD §21.4).

## Scope boundaries (what NOT to add)

- **No `govulncheck` target** — not in the 6-target contract; CI matrix (P1.M5.T3.S1) owns it.
- **No coverage ≥85% gate** — that is P1.M5.T3.S3. This subtask's `coverage` only RUNS + reports.
- **No `.golangci.yml`** — not requested; linters are configured later / per CI.
- **No goreleaser, no release target** — P1.M5.T3.S2.
- **No CI workflow files** — P1.M5.T3.S1.

## Sources

- Go ldflags `-X`: <https://pkg.go.dev/cmd/link> — "-X importpath.name=value: Set the value of a
  string variable… It is safe to use -X on a symbol that does not exist" (verified empirically above).
- GNU Make manual (PHONY, `?=`, recipes/tabs): <https://www.gnu.org/software/make/manual/html_node/>
- golangci-lint install: <https://golangci-lint.run/usage/install/#binary-installation>
- PRD §21.1 (Build), §21.4 (Versioning), §20.3 (Coverage target), §20.4 (CI matrix).
