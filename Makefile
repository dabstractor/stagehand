## Stagehand — build, test, lint, coverage, and install targets. (PRD §21.1)
##
## Usage:  make <target>
##
## Targets:
##   build          Compile the stagehand binary to ./bin/stagehand
##   install        Install stagehand into $GOPATH/bin
##   build-test     Compile a test binary as ./bin/stagehand-test (side-by-side, same source)
##   install-test   Install stagehand-test where the official binary lives (on PATH, like `stagehand`)
##   test           Run all tests with the race detector enabled
##   coverage       Run tests and print per-function coverage
##   coverage-gate  Fail if any of internal/{git,provider,generate,config} < 85% (PRD §20.3)
##   lint           Run golangci-lint
##   clean          Remove bin/, coverage.out, and dist/
##   help           Print this help

.DEFAULT_GOAL := build

# --- Version (PRD §21.1, §21.4) -------------------------------------------------
# Injected into main.version via -ldflags at build time. Defaults to "dev";
# override for releases:  make build VERSION=v1.2.3   (goreleaser sets it via env).
# NOTE: -X main.version=... is a silent no-op until main.go declares `var version string`
# (a later subtask adds it). VERIFIED: build exits 0 either way.
VERSION ?= dev

# --- Paths & flags --------------------------------------------------------------
BIN_DIR  := bin
BIN      := $(BIN_DIR)/stagehand
BIN_TEST := $(BIN_DIR)/stagehand-test
MAIN_PKG := ./cmd/stagehand
LDFLAGS  := -X main.version=$(VERSION)

# --- Install dir for the -test variant ----------------------------------------
# The official `make install` runs `go install`, dropping the binary in $GOBIN
# (defaults to $GOPATH/bin). On PATH it is reached via a ~/.local/bin/stagehand symlink.
# `go install` can't rename, so install-test mirrors that exactly: build -> $GOBIN/stagehand-test,
# then a matching ~/.local/bin/stagehand-test symlink puts it on PATH.
GOBIN ?= $(shell go env GOBIN)
ifeq ($(strip $(GOBIN)),)
GOBIN := $(shell go env GOPATH)/bin
endif

# --- Coverage gate (PRD §20.3) -------------------------------------------------
# Statement-weighted per-package floor on the 4 core packages. internal/ui has a lower bar
# (PRD §20.3 — hard to test, low risk) and is intentionally excluded. Runnable locally and in CI.
MODULE            := $(shell go list -m)
COVERAGE_THRESHOLD ?= 85
COVERAGE_PKGS     := ./internal/git/... ./internal/provider/... ./internal/generate/... ./internal/config/...

.PHONY: build build-test install install-test test coverage coverage-gate lint clean help

build: ## Compile the stagehand binary to ./bin/stagehand
	go build -ldflags "$(LDFLAGS)" -o $(BIN) $(MAIN_PKG)

build-test: ## Compile a test binary as ./bin/stagehand-test (side-by-side with the real one)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_TEST) $(MAIN_PKG)

install: ## Install stagehand into $GOPATH/bin
	go install -ldflags "$(LDFLAGS)" $(MAIN_PKG)

install-test: build-test ## Install stagehand-test where the official binary lives (on PATH, like `stagehand`)
	@mkdir -p "$(GOBIN)" "$(HOME)/.local/bin"
	install -m 0755 "$(BIN_TEST)" "$(GOBIN)/stagehand-test"
	ln -sfn "$(GOBIN)/stagehand-test" "$(HOME)/.local/bin/stagehand-test"
	@echo "installed → $(GOBIN)/stagehand-test   (PATH: ~/.local/bin/stagehand-test)"

test: ## Run all tests with the race detector enabled
	go test -race ./...

coverage: ## Run tests and print per-function coverage
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

coverage-gate: ## Enforce >=85% statement coverage on internal/{git,provider,generate,config} (PRD §20.3)
	@echo "coverage gate: threshold=$(COVERAGE_THRESHOLD)% on the 4 core packages (PRD §20.3)"
	go test -coverprofile=coverage.out $(COVERAGE_PKGS)
	@awk -v threshold=$(COVERAGE_THRESHOLD) -v mod='$(MODULE)' '\
	  /^mode:/ { next } \
	  { \
	    f=$$1; sub(/:[0-9]+.*$$/, "", f); \
	    n=split(f, parts, "/"); pkg=""; for (i=1; i<n; i++) pkg = pkg (i>1 ? "/" : "") parts[i]; \
	    tot[pkg]+=$$2; if ($$3+0 > 0) cov[pkg]+=$$2; \
	  } \
	  END { \
	    t[1]=mod "/internal/git"; t[2]=mod "/internal/provider"; \
	    t[3]=mod "/internal/generate"; t[4]=mod "/internal/config"; \
	    fail=0; \
	    for (i=1; i<=4; i++) { \
	      if (!(t[i] in tot)) { printf("  ERROR  %-58s no coverage data\n", t[i]); fail=1; continue } \
	      pct = (tot[t[i]]>0) ? cov[t[i]]/tot[t[i]]*100 : 0; \
	      mark = (pct+0 >= threshold) ? "OK" : "FAIL"; \
	      printf("  %-58s %5.1f%%  %s\n", t[i], pct, mark); \
	      if (pct+0 < threshold) { printf("           >>> %s coverage %.1f%% < %d%% threshold (PRD §20.3)\n", t[i], pct, threshold); fail=1 } \
	    } \
	    if (fail) { printf("  coverage gate: FAIL (one or more packages below %d%%)\n", threshold) } \
	    else      { printf("  coverage gate: PASS (all 4 packages >= %d%%)\n", threshold) }; \
	    exit fail \
	  }' coverage.out

lint: ## Run golangci-lint
	golangci-lint run

clean: ## Remove bin/, coverage.out, and dist/
	rm -rf $(BIN_DIR) coverage.out dist/

help: ## Print this help
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'
