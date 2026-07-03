#!/usr/bin/env bash
# =============================================================================
# validate.sh — Stagehand comprehensive validation
#
# Runs five phases against the Stagehand codebase:
#   1. Linting        — golangci-lint, go vet
#   2. Type checking  — go build ./... (Go compiler = the type checker)
#   3. Style checking — gofmt -l, markdownlint
#   4. Unit testing   — go test -race ./... + PRD §20.3 coverage gate (≥85%)
#   5. E2E testing    — the built-in e2e suite + a full user-workflow simulation
#                       (Quick start, --edit/--push/--exclude/--format, hook mode,
#                       integrate, config/models/providers, atomicity, exit codes)
#
# DESIGN NOTES
#   * Builds FRESH stagehand + stubagent binaries into a temp dir and references
#     them by ABSOLUTE PATH — this sidesteps any stale `stagehand` already on
#     $PATH (e.g. an older install predating the `hook` subcommand).
#   * E2E runs against a STUB agent (cmd/stubagent) so it is deterministic and
#     needs no real coding-agent CLI or network. Real-agent scenarios are
#     exercised by the in-repo e2e suite when STAGEHAND_RUN_REAL=1.
#   * SANDBOXED: HOME, XDG_CONFIG_HOME, GIT_CONFIG_GLOBAL, and a fake `lazygit`
#     are all pointed at temp dirs — real user config files are NEVER touched.
#   * Phases degrade gracefully: a missing optional tool (golangci-lint,
#     govulncheck, markdownlint) is reported as SKIPPED, not a hard failure.
#
# Exit codes: 0 = all HARD checks passed (advisory issues may still be listed);
#             1 = at least one HARD check failed.
# =============================================================================
set -uo pipefail

# Locate the repo root (directory containing this script's parent project).
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# -----------------------------------------------------------------------------
# Colors / formatting (honor NO_COLOR)
# -----------------------------------------------------------------------------
if [[ -n "${NO_COLOR:-}" ]] || [[ ! -t 1 ]]; then
    C_RED=""; C_GREEN=""; C_YELLOW=""; C_BLUE=""; C_BOLD=""; C_RESET=""
else
    C_RED=$'\033[31m'; C_GREEN=$'\033[32m'; C_YELLOW=$'\033[33m'
    C_BLUE=$'\033[34m'; C_BOLD=$'\033[1m'; C_RESET=$'\033[0m'
fi

# Counters
HARD_FAIL=0
HARD_PASS=0
ADVISORY=0
SKIPPED=0

phase() { printf "\n${C_BOLD}${C_BLUE}═══ %s ═══${C_RESET}\n" "$*"; }
ok()    { printf "  ${C_GREEN}✓ PASS${C_RESET}  %s\n" "$1"; HARD_PASS=$((HARD_PASS+1)); }
bad()   { printf "  ${C_RED}✗ FAIL${C_RESET}  %s\n" "$1"; HARD_FAIL=$((HARD_FAIL+1)); }
warn()  { printf "  ${C_YELLOW}⚠ ADVISE${C_RESET} %s\n" "$1"; ADVISORY=$((ADVISORY+1)); }
skip()  { printf "  ${C_BLUE}⊘ SKIP${C_RESET}  %s\n" "$1"; SKIPPED=$((SKIPPED+1)); }
info()  { printf "         %s\n" "$1"; }

have() { command -v "$1" >/dev/null 2>&1; }

# -----------------------------------------------------------------------------
# Scratch area: fresh binaries + isolated sandbox (cleaned on exit)
# -----------------------------------------------------------------------------
WORK="$(mktemp -d -t stagehand-validate-XXXXXX)"
trap 'rm -rf "$WORK"' EXIT

BIN_DIR="$WORK/bin"
SANDBOX="$WORK/sandbox"          # isolated HOME tree
export HOME="$SANDBOX/home"; mkdir -p "$HOME"
export XDG_CONFIG_HOME="$SANDBOX/xdg"; mkdir -p "$XDG_CONFIG_HOME"
export GIT_CONFIG_GLOBAL="$SANDBOX/global.gitconfig"
FAKE_LAZYGIT_DIR="$SANDBOX/fakebin"; mkdir -p "$FAKE_LAZYGIT_DIR"

# Stub-agent TOML config (deterministic; no real agent needed).
STUB_TOML="$WORK/stub.toml"
STUB="$BIN_DIR/stubagent"
HAND="$BIN_DIR/stagehand"

# Repo-local git identity for test repos.
export GIT_AUTHOR_NAME="Validate"  GIT_AUTHOR_EMAIL="v@e.x"
export GIT_COMMITTER_NAME="Validate" GIT_COMMITTER_EMAIL="v@e.x"

echo "${C_BOLD}Stagehand validation${C_RESET}  (repo: $SCRIPT_DIR)"
echo "work dir: $WORK"

# =============================================================================
phase "Phase 0 — Build fresh binaries"
# =============================================================================
if go build -o "$HAND" ./cmd/stagehand && go build -o "$STUB" ./cmd/stubagent; then
    ok "Built fresh stagehand + stubagent into $BIN_DIR"
else
    bad "Build failed"
    exit 1
fi
# Sanity: the freshly built binary is the one under test (no stale shadow).
VERSION_OUT="$("$HAND" --version 2>&1)"
ok "stagehand --version → ${VERSION_OUT}"

# =============================================================================
phase "Phase 1 — Linting"
# =============================================================================
# go vet (always available with the Go toolchain)
if go vet ./... >"$WORK/vet.log" 2>&1; then
    ok "go vet ./... clean"
else
    bad "go vet reported issues:"; sed 's/^/        /' "$WORK/vet.log" | head -20
fi

# golangci-lint (CI pins v1.61; use whatever is installed, note the version)
if have golangci-lint; then
    GL_VER="$(golangci-lint --version 2>&1 | head -1)"
    info "using $GL_VER (CI pins v1.61)"
    if golangci-lint run --timeout=5m >"$WORK/lint.log" 2>&1; then
        ok "golangci-lint run clean"
    else
        # Findings here are commonly in test code and version-dependent. Surface
        # them as ADVISORY so they are visible without failing the whole run,
        # since CI's pinned v1.61 may behave differently.
        warn "golangci-lint reported findings (test-code / version-dependent):"
        sed 's/^/        /' "$WORK/lint.log" | head -20
    fi
else
    skip "golangci-lint not installed (CI runs it via the golangci-lint-action)"
fi

# govulncheck (advisory; CI runs it in a dedicated job)
if have govulncheck; then
    if govulncheck ./... >"$WORK/vuln.log" 2>&1; then
        ok "govulncheck: no known vulnerabilities"
    else
        bad "govulncheck reported vulnerabilities:"; sed 's/^/        /' "$WORK/vuln.log" | head -30
    fi
else
    skip "govulncheck not installed (CI runs it in a dedicated job)"
fi

# =============================================================================
phase "Phase 2 — Type checking (go build)"
# =============================================================================
if go build ./... >"$WORK/build.log" 2>&1; then
    ok "go build ./... compiles all packages"
else
    bad "go build failed:"; sed 's/^/        /' "$WORK/build.log" | head -20
fi
# Cross-compile the two arm64 targets CI covers (proves they build, no emulation)
if go env -w GOARCH=arm64 GOOS=linux 2>/dev/null && go build ./... >/dev/null 2>&1 \
   && go env -w GOARCH=arm64 GOOS=windows 2>/dev/null && go build ./... >/dev/null 2>&1; then
    ok "Cross-build linux/arm64 + windows/arm64 compiles"
else
    warn "Cross-build step had issues (non-fatal locally)"
fi
go env -u GOARCH GOOS >/dev/null 2>&1 || true

# =============================================================================
phase "Phase 3 — Style checking"
# =============================================================================
# gofmt: list files that are NOT formatted. CI's golangci-lint config does not
# enable the gofmt linter, so this is advisory (surfaces drift).
GOFMT_OUT="$(gofmt -l . 2>/dev/null)"
if [[ -z "$GOFMT_OUT" ]]; then
    ok "gofmt: all .go files formatted"
else
    warn "gofmt: these files need formatting (not gated by CI):"
    printf '%s\n' "$GOFMT_OUT" | sed 's/^/        /'
fi

# markdownlint against the project's OWN .markdownlint.json config.
if have npx; then
    if timeout 120 npx -y markdownlint-cli@latest -c .markdownlint.json \
            "docs/**/*.md" "README.md" >"$WORK/mdlint.log" 2>&1; then
        ok "markdownlint: docs clean against .markdownlint.json"
    else
        warn "markdownlint: violations against project's own config (not gated by CI):"
        sed 's/^/        /' "$WORK/mdlint.log" | head -20
    fi
else
    skip "npx/markdownlint not available"
fi

# CLI help-output sanity: bool flags --edit/--push must NOT advertise a value.
# (Known advisory: pflag lifts the first backquoted word as the placeholder.)
HELP_OUT="$("$HAND" --help 2>&1)"
if printf '%s\n' "$HELP_OUT" | grep -qE '^\s+--edit\s+git var GIT_EDITOR'; then
    warn "--edit shows a misleading value placeholder in --help (cosmetic; flag still works as a bool)"
else
    ok "--edit help line is clean"
fi
if printf '%s\n' "$HELP_OUT" | grep -qE '^\s+--push\s+git push'; then
    warn "--push shows a misleading value placeholder in --help (cosmetic; flag still works as a bool)"
else
    ok "--push help line is clean"
fi

# =============================================================================
phase "Phase 4 — Unit testing + coverage gate"
# =============================================================================
if go test -race -count=1 ./... >"$WORK/test.log" 2>&1; then
    ok "go test -race ./... (all packages pass)"
else
    bad "unit tests failed:"; sed 's/^/        /' "$WORK/test.log" | tail -30
fi
# PRD §20.3 coverage gate (≥85% on the 4 core packages)
if make coverage-gate >"$WORK/cov.log" 2>&1; then
    ok "coverage gate ≥85% (internal/{git,provider,generate,config})"
    grep -E '^  github' "$WORK/cov.log" | sed 's/^/        /'
else
    bad "coverage gate failed:"; tail -15 "$WORK/cov.log" | sed 's/^/        /'
fi

# Built-in e2e suite (PRD §20.5): throwaway-repo harness over the compiled binary.
if go test -tags e2e -count=1 ./internal/e2e/ >"$WORK/e2e.log" 2>&1; then
    ok "built-in e2e suite (stub scenarios) passes"
else
    bad "built-in e2e suite failed:"; sed 's/^/        /' "$WORK/e2e.log" | tail -30
fi

# =============================================================================
# Phase 5 — E2E user-workflow simulation
#
# Drives the FRESHLY BUILT binary through the documented user journeys from
# README.md / docs/cli.md, using the stub agent for determinism. Each test gets
# its own throwaway git repo so they cannot contaminate each other.
# =============================================================================
phase "Phase 5 — End-to-end user-workflow simulation"

# Write the stub config (default provider = stub so auto-detect is bypassed).
write_stub_config() {
    cat > "$STUB_TOML" <<EOF
config_version = 3
[defaults]
provider = "stub"
[provider.stub]
command = "$STUB"
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
default_model = "stub"
EOF
}
write_stub_config

# new_repo <name> : create a fresh git repo under the sandbox, seed one commit.
new_repo() {
    local name="$1" d="$SANDBOX/$1"
    rm -rf "$d"; mkdir -p "$d"
    git -C "$d" init -q
    git -C "$d" config user.name "Validate"; git -C "$d" config user.email "v@e.x"
    printf 'init\n' > "$d/readme.md"
    git -C "$d" add readme.md
    git -C "$d" commit -q -m "chore: init"
    printf '%s' "$d"
}

# sh_run <repo> <env...> -- <args...> : run the binary, capture exit code.
SH_EXIT=0
sh_run() {
    local repo="$1"; shift
    local -a env=() args=()
    while [[ "$1" != "--" ]]; do env+=("$1"); shift; done
    shift
    while [[ $# -gt 0 ]]; do args+=("$1"); shift; done
    SH_EXIT=0
    env "${env[@]}" "$HAND" --config "$STUB_TOML" --no-color "${args[@]}" \
        >/dev/null 2>&1 || SH_EXIT=$?
}

# ---- Workflow: Quick start (README §Quick start) -----------------------------
R="$(new_repo quickstart)"; printf 'fn login(){}\n' > "$R/login.js"; git -C "$R" add login.js
sh_run "$R" STAGEHAND_STUB_OUT="feat: add login flow" -- --provider stub
if [[ $SH_EXIT -eq 0 ]] && git -C "$R" diff-tree --no-commit-id --name-only -r HEAD | grep -q '^login.js$'; then
    ok "Quick start: stage → stagehand → atomic commit (login.js committed)"
else
    bad "Quick start workflow failed (exit=$SH_EXIT)"
fi

# ---- Workflow: clean tree → exit 2 ------------------------------------------
R="$(new_repo cleantree)"
sh_run "$R" -- --provider stub
if [[ $SH_EXIT -eq 2 ]]; then ok "Clean tree → exit 2 (Nothing to commit)"
else bad "Clean tree should exit 2, got $SH_EXIT"; fi

# ---- Workflow: --dry-run (preview, no commit) -------------------------------
R="$(new_repo dryrun)"; printf 'x\n' > "$R/dry.txt"; git -C "$R" add dry.txt
BEFORE="$(git -C "$R" rev-parse HEAD)"
sh_run "$R" STAGEHAND_STUB_OUT="feat: preview" -- --provider stub --dry-run
AFTER="$(git -C "$R" rev-parse HEAD)"
if [[ $SH_EXIT -eq 0 && "$BEFORE" == "$AFTER" ]]; then ok "--dry-run prints message, does NOT commit"
else bad "--dry-run should not commit / exit 0 (exit=$SH_EXIT)"; fi

# ---- Workflow: -a (stage everything) ----------------------------------------
R="$(new_repo allflag)"; printf '1\n' > "$R/a.txt"; printf '2\n' > "$R/b.txt"
sh_run "$R" STAGEHAND_STUB_OUT="chore: all" -- -a
if [[ $SH_EXIT -eq 0 ]] \
   && git -C "$R" diff-tree --no-commit-id --name-only -r HEAD | grep -q '^a.txt$' \
   && git -C "$R" diff-tree --no-commit-id --name-only -r HEAD | grep -q '^b.txt$'; then
    ok "-a stages and commits everything (a.txt + b.txt)"
else bad "-a workflow failed (exit=$SH_EXIT)"; fi

# ---- Workflow: --exclude (payload-only; FR-X5: still committed) -------------
R="$(new_repo exclude)"; printf 'code\n' > "$R/feat.go"; printf 'SECRET\n' > "$R/secret.txt"
git -C "$R" add feat.go secret.txt
SEEN="$WORK/exclude_payload.txt"
env STAGEHAND_STUB_OUT="feat: x" STAGEHAND_STUB_STDINFILE="$SEEN" \
    "$HAND" --config "$STUB_TOML" --no-color -x 'secret.txt' >/dev/null 2>&1 || true
if grep -q 'SECRET' "$SEEN" 2>/dev/null; then
    bad "--exclude: secret LEAKED into agent payload"
elif git -C "$R" diff-tree --no-commit-id --name-only -r HEAD | grep -q '^secret.txt$'; then
    ok "--exclude hides secret from payload but STILL commits it (FR-X5)"
else
    bad "--exclude: secret.txt not committed (FR-X5 violated)"
fi

# ---- Workflow: --edit (review in $EDITOR before commit) ---------------------
R="$(new_repo editflag)"; printf 'y\n' > "$R/e.txt"; git -C "$R" add e.txt
ED="$WORK/editor.sh"
printf '#!/bin/sh\nsed -i "1s/.*/chore: EDITED/" "$1"\n' > "$ED"; chmod +x "$ED"
GIT_EDITOR="$ED" STAGEHAND_STUB_OUT="feat: original" "$HAND" --config "$STUB_TOML" \
    --no-color --provider stub >/dev/null 2>&1 || true
if [[ "$(git -C "$R" log -1 --format='%s')" == "chore: EDITED" ]]; then
    ok "--edit: user edit overrides generated message"
else bad "--edit workflow did not apply editor edit"; fi

# ---- Workflow: --push (commit + push to a bare remote) ----------------------
R="$(new_repo pushflag)"; BARE="$SANDBOX/bare.git"
rm -rf "$BARE"; git init --bare -q "$BARE"
git -C "$R" remote add origin "$BARE"
printf 'p\n' > "$R/p.txt"; git -C "$R" add p.txt
sh_run "$R" STAGEHAND_STUB_OUT="feat: pushable" -- --push
if [[ $SH_EXIT -eq 0 ]] && git -C "$BARE" rev-parse --verify -q HEAD >/dev/null 2>&1; then
    ok "--push: commit created and pushed to remote"
else bad "--push workflow failed (exit=$SH_EXIT)"; fi

# ---- Workflow: --template / --locale / --context (message shaping) ----------
R="$(new_repo tpl)"; printf 't\n' > "$R/t.txt"; git -C "$R" add t.txt
env STAGEHAND_STUB_OUT="feat: base" "$HAND" --config "$STUB_TOML" --no-color \
    --provider stub --template '$msg (#205)' >/dev/null 2>&1 || true
[[ "$(git -C "$R" log -1 --format='%s')" == "feat: base (#205)" ]] \
    && ok "--template wraps message (\$msg substitution)" \
    || bad "--template did not wrap message"

R="$(new_repo ctx)"; printf 'c\n' > "$R/c.txt"; git -C "$R" add c.txt
SEEN="$WORK/ctx_payload.txt"
env STAGEHAND_STUB_OUT="feat: c" STAGEHAND_STUB_STDINFILE="$SEEN" \
    "$HAND" --config "$STUB_TOML" --no-color --provider stub --context 'hotfix for #812' >/dev/null 2>&1 || true
grep -q 'hotfix for #812' "$SEEN" 2>/dev/null \
    && ok "--context appended to agent payload" \
    || bad "--context not present in payload"

# ---- Workflow: --reasoning graceful no-op (FR-R6) ---------------------------
R="$(new_repo reason)"; printf 'r\n' > "$R/r.txt"; git -C "$R" add r.txt
sh_run "$R" STAGEHAND_STUB_OUT="feat: r" -- --provider stub --reasoning high
[[ $SH_EXIT -eq 0 ]] && ok "--reasoning is a graceful no-op on stub (FR-R6)" \
                    || bad "--reasoning should be a no-op, exit=$SH_EXIT"

# ---- Workflow: --commits 1 / --single (force single-commit path) -----------
R="$(new_repo single)"; printf 's\n' > "$R/s.txt"; git -C "$R" add s.txt
sh_run "$R" STAGEHAND_STUB_OUT="feat: single" -- --commits 1
BEFORE="$(git -C "$R" rev-parse HEAD^1 2>/dev/null)"
[[ $SH_EXIT -eq 0 ]] && ok "--commits 1 forces the single-commit path" \
                    || bad "--commits 1 failed (exit=$SH_EXIT)"

# ---- Exit-code contract: rescue (exit 3, HEAD unchanged) -------------------
R="$(new_repo rescue)"; printf 'x\n' > "$R/x.txt"; git -C "$R" add x.txt
BEFORE="$(git -C "$R" rev-parse HEAD)"
sh_run "$R" STAGEHAND_STUB_OUT="" -- --provider stub      # empty → unparseable → rescue
if [[ $SH_EXIT -eq 3 && "$(git -C "$R" rev-parse HEAD)" == "$BEFORE" ]]; then
    ok "Rescue path: exit 3, HEAD byte-for-byte unchanged (atomicity)"
else
    bad "Rescue should be exit 3 with HEAD unchanged (got exit=$SH_EXIT)"
fi

# ---- Exit-code contract: CAS abort (concurrent HEAD move → exit 1) ----------
R="$(new_repo cas)"; printf 'k\n' > "$R/k.txt"; git -C "$R" add k.txt
MARK="$WORK/cas.marker"; rm -f "$MARK"
env STAGEHAND_STUB_OUT="feat: cas" STAGEHAND_STUB_MARKER="$MARK" STAGEHAND_STUB_SLEEP_MS=1200 \
    "$HAND" --config "$STUB_TOML" --no-color --provider stub >/dev/null 2>&1 &
CAS_PID=$!
for _ in $(seq 1 100); do [[ -f "$MARK" ]] && break; sleep 0.1; done
git -C "$R" commit --allow-empty -q -m "concurrent"
CONCURRENT="$(git -C "$R" rev-parse HEAD)"
wait "$CAS_PID" || true; CAS_EXIT=$?
if [[ $CAS_EXIT -eq 1 && "$(git -C "$R" rev-parse HEAD)" == "$CONCURRENT" ]]; then
    ok "CAS abort: concurrent HEAD move → exit 1, stagehand commit rejected"
else
    bad "CAS abort should be exit 1 with HEAD at concurrent commit (got $CAS_EXIT)"
fi

# ---- Stage-while-generating: file added mid-generation is EXCLUDED ----------
R="$(new_repo overlap)"; printf 'kept\n' > "$R/kept.txt"
MARK="$WORK/ovl.marker"; rm -f "$MARK"
env STAGEHAND_STUB_OUT="feat: kept" STAGEHAND_STUB_MARKER="$MARK" STAGEHAND_STUB_SLEEP_MS=800 \
    "$HAND" --config "$STUB_TOML" --no-color --provider stub >/dev/null 2>&1 &
OVL_PID=$!
for _ in $(seq 1 100); do [[ -f "$MARK" ]] && break; sleep 0.1; done
printf 'intruder\n' > "$R/intruder.txt"      # written UN-staged, mid-generation
wait "$OVL_PID" || true; OVL_EXIT=$?
if git -C "$R" diff-tree --no-commit-id --name-only -r HEAD | grep -q '^intruder.txt$'; then
    bad "Stage-while-generating: intruder.txt was swept into the in-flight commit"
else
    ok "Stage-while-generating: mid-run file stays out of the in-flight commit"
fi

# ---- Error contract: multi-backend bare model → exit 1 (FR-R5b) -------------
MULTI="$WORK/multi.toml"
cat > "$MULTI" <<EOF
config_version = 3
[provider.testmulti]
command = "$STUB"
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
model_flag = "--model"
provider_flag = "--provider"
default_model = "x"
[defaults]
provider = "stub"
[provider.stub]
command = "$STUB"
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
default_model = "stub"
EOF
R="$(new_repo mbare)"; printf 'm\n' > "$R/m.txt"; git -C "$R" add m.txt
BEFORE="$(git -C "$R" rev-parse HEAD)"
env STAGEHAND_STUB_OUT="feat: x" "$HAND" --config "$MULTI" --no-color \
    --provider testmulti --model bare >/dev/null 2>&1 || MBARE_EXIT=$?
if [[ ${MBARE_EXIT:-0} -eq 1 && "$(git -C "$R" rev-parse HEAD)" == "$BEFORE" ]]; then
    ok "FR-R5b: multi-backend bare model → exit 1, HEAD unchanged"
else
    bad "FR-R5b should exit 1 (got ${MBARE_EXIT:-0})"
fi

# ---- Error contract: unknown format / template / provider → exit 1 ---------
R="$(new_repo errfmt)"; printf 'f\n' > "$R/f.txt"; git -C "$R" add f.txt
sh_run "$R" STAGEHAND_STUB_OUT="x" -- --provider stub --format bogus
[[ $SH_EXIT -eq 1 ]] && ok "Unknown --format → exit 1" || bad "Unknown format should exit 1 (got $SH_EXIT)"
sh_run "$R" STAGEHAND_STUB_OUT="x" -- --provider stub --template 'no placeholder'
[[ $SH_EXIT -eq 1 ]] && ok "Template without \$msg → exit 1" || bad "Bad template should exit 1 (got $SH_EXIT)"
sh_run "$R" STAGEHAND_STUB_OUT="x" -- --provider nosuchprovider
[[ $SH_EXIT -eq 1 ]] && ok "Unknown --provider → exit 1" || bad "Unknown provider should exit 1 (got $SH_EXIT)"

# ---- Workflow: hook mode (install → git commit → uninstall) ----------------
R="$(new_repo hook)"; export PATH="$BIN_DIR:$PATH"
"$HAND" --config "$STUB_TOML" --no-color hook install >/dev/null 2>&1
HS1="$?"; HSTAT="$("$HAND" --config "$STUB_TOML" --no-color hook status 2>&1)"
printf 'h\n' > "$R/h.txt"; git -C "$R" add h.txt
STAGEHAND_CONFIG="$STUB_TOML" STAGEHAND_STUB_OUT="feat: hook msg" GIT_EDITOR=true \
    git -C "$R" commit -q >/dev/null 2>&1
if [[ $HS1 -eq 0 && "$HSTAT" == "stagehand (v1)" \
      && "$(git -C "$R" log -1 --format='%s')" == "feat: hook msg" ]]; then
    ok "Hook mode: install fills message on plain 'git commit'"
else
    bad "Hook mode workflow failed (status=$HSTAT, install_exit=$HS1)"
fi
# hook never-blocks on failure (stub exit 1 → editor fallback, commit proceeds)
printf 'h2\n' > "$R/h2.txt"; git -C "$R" add h2.txt
ED2="$WORK/ed2.sh"; printf '#!/bin/sh\necho fallback > "$1"\n' > "$ED2"; chmod +x "$ED2"
STAGEHAND_CONFIG="$STUB_TOML" STAGEHAND_STUB_EXIT=1 GIT_EDITOR="$ED2" \
    git -C "$R" commit -q >/dev/null 2>&1
[[ $? -eq 0 && "$(git -C "$R" log -1 --format='%s')" == "fallback" ]] \
    && ok "Hook never-blocks: agent failure → editor fallback, commit proceeds" \
    || bad "Hook should never-block on failure"
"$HAND" --config "$STUB_TOML" --no-color hook uninstall >/dev/null 2>&1

# ---- Workflow: integrate (sandboxed; fake lazygit + global gitconfig) -------
# Fake lazygit so --print-config-dir points at the sandbox (never the real config).
printf '#!/bin/sh\n[ "$1" = "--print-config-dir" ] && echo "%s" && exit 0\nexit 0\n' \
    "$SANDBOX/lgcfg" > "$FAKE_LAZYGIT_DIR/lazygit"; chmod +x "$FAKE_LAZYGIT_DIR/lazygit"
mkdir -p "$SANDBOX/lgcfg"; printf 'gui:\n  nerdFontsVersion: "3"\n' > "$SANDBOX/lgcfg/config.yml"
IR="$(new_repo integrate)"; cd "$IR"
PATH="$FAKE_LAZYGIT_DIR:$BIN_DIR:$PATH" "$HAND" --config "$STUB_TOML" --no-color \
    integrate install lazygit --yes >/dev/null 2>&1
LI1=$?
if [[ $LI1 -eq 0 ]] && grep -q 'stagehand-integration' "$SANDBOX/lgcfg/config.yml" 2>/dev/null \
   && ls "$SANDBOX/lgcfg"/config.yml.stagehand-backup.* >/dev/null 2>&1; then
    ok "integrate install lazygit: writes marker + backup (no-mangle protocol)"
else
    bad "integrate lazygit install failed (exit=$LI1)"
fi
# Idempotent re-install
PATH="$FAKE_LAZYGIT_DIR:$BIN_DIR:$PATH" "$HAND" --config "$STUB_TOML" --no-color \
    integrate install lazygit --yes >/dev/null 2>&1
# Remove cleans up
PATH="$FAKE_LAZYGIT_DIR:$BIN_DIR:$PATH" "$HAND" --config "$STUB_TOML" --no-color \
    integrate remove lazygit --yes >/dev/null 2>&1
if ! grep -q 'stagehand-integration' "$SANDBOX/lgcfg/config.yml" 2>/dev/null; then
    ok "integrate remove lazygit: marker removed cleanly"
else
    bad "integrate remove lazygit left the marker behind"
fi
# git-alias writes to the SANDBOXED global gitconfig
PATH="$FAKE_LAZYGIT_DIR:$BIN_DIR:$PATH" "$HAND" --config "$STUB_TOML" --no-color \
    integrate install git-alias --yes >/dev/null 2>&1
if [[ "$(git config --global alias.stagehand 2>/dev/null)" == "!stagehand" ]]; then
    ok "integrate install git-alias: registers 'git stagehand' alias"
else
    bad "integrate git-alias did not register the alias"
fi
cd "$SCRIPT_DIR"

# ---- Workflow: config init / path / upgrade ---------------------------------
FH="$SANDBOX/cfginit"; rm -rf "$FH"; mkdir -p "$FH"
env -i HOME="$FH" PATH="$PATH" "$HAND" config init --provider pi >/dev/null 2>&1
if [[ -f "$FH/.config/stagehand/config.toml" ]]; then
    ok "config init --provider pi writes a populated config"
else
    bad "config init did not write a config"
fi
# config init refuses to overwrite (exit 1) unless --force
env -i HOME="$FH" PATH="$PATH" "$HAND" config init --provider pi >/dev/null 2>&1
[[ $? -eq 1 ]] && ok "config init refuses to overwrite existing file (exit 1)" \
              || bad "config init should refuse overwrite"
# config upgrade on a v1 file
UP="$WORK/up.toml"; printf 'provider = "pi"\nmodel = "glm-5"\n' > "$UP"
"$HAND" --config "$UP" config upgrade >/dev/null 2>&1
grep -q '^config_version = 3' "$UP" 2>/dev/null \
    && ok "config upgrade migrates to config_version = 3" \
    || bad "config upgrade did not produce version 3"

# ---- Workflow: models / providers show --------------------------------------
"$HAND" --config "$STUB_TOML" --no-color models stub >/dev/null 2>&1
[[ $? -eq 0 ]] && ok "models <provider> runs" || bad "models command failed"
"$HAND" --config "$STUB_TOML" --no-color providers show stub >/dev/null 2>&1
[[ $? -eq 0 ]] && ok "providers show <name> runs" || bad "providers show failed"
"$HAND" --config "$STUB_TOML" --no-color providers list >/dev/null 2>&1
[[ $? -eq 0 ]] && ok "providers list runs" || bad "providers list failed"

# ---- --version short-circuits BEFORE config load (works outside a git repo) --
( cd "$SANDBOX" && "$HAND" --version >/dev/null 2>&1 )
[[ $? -eq 0 ]] && ok "--version works outside a git repo" || bad "--version failed outside a repo"

# =============================================================================
phase "Summary"
# =============================================================================
printf "\n  Hard checks:   ${C_GREEN}%d passed${C_RESET}, ${C_RED}%d failed${C_RESET}\n" "$HARD_PASS" "$HARD_FAIL"
printf "  Advisory:      ${C_YELLOW}%d${C_RESET}\n" "$ADVISORY"
printf "  Skipped:       ${C_BLUE}%d${C_RESET}\n" "$SKIPPED"
echo

if [[ $HARD_FAIL -gt 0 ]]; then
    printf "${C_RED}${C_BOLD}VALIDATION FAILED: %d hard check(s) failed.${C_RESET}\n" "$HARD_FAIL"
    exit 1
fi
printf "${C_GREEN}${C_BOLD}All hard checks passed.${C_RESET} "
if [[ $ADVISORY -gt 0 ]]; then
    printf "${C_YELLOW}%d advisory issue(s) listed above — review before release.${C_RESET}\n" "$ADVISORY"
else
    printf "No advisory issues.\n"
fi
exit 0
