#!/usr/bin/env bash
# validate.sh — Stagehand comprehensive validation (PRD-aligned).
#
# Runs every validation phase that exists in this codebase:
#   Phase 1: Linting        — golangci-lint (+ go vet fallback)
#   Phase 2: Type checking  — go vet ./... (Go's static type/analysis checker)
#   Phase 3: Style checking — gofmt -l
#   Phase 4: Unit testing   — go test -race ./...  +  -tags e2e stub scenarios
#   Phase 5: Coverage gate  — >=85% on internal/{git,provider,generate,config} (PRD §20.3)
#   Phase 6: Vulnerability  — govulncheck (CI runs this)
#   Phase 7: E2E workflows  — the REAL documented user journeys via cmd/stubagent
#                             (single commit, -a, --dry-run, --single, --commits 1,
#                              rescue, dedupe retry, fail-fast, FR-R5b bare-model,
#                              binary filtering, config init/upgrade/path, providers,
#                              decompose planner single-shortcut)
#
# Exit code: 0 only if EVERY phase passes. Non-zero on any failure.
#
# USAGE:
#   ./validate.sh              # run all phases
#   ./validate.sh --no-e2e     # skip the Phase-7 shell workflows (faster; still lints/tests)
#
# NOTES:
#   * Self-contained: builds stagehand + stubagent into a temp dir, uses a throwaway git repo.
#   * Deterministic: the stub agent (cmd/stubagent) replaces real agents — no network, no quota.
#   * The PRD headline feature (multi-commit decompose with a REAL tooled stager) is exercised
#     end-to-end by the Go test suite (internal/e2e + internal/decompose) and by opt-in
#     STAGEHAND_RUN_REAL=1 runs; this script covers the stub-reachable surface deterministically.
#   * Lint version caveat: CI pins golangci-lint v1.61 + Go 1.22. If the local toolchain is newer
#     (e.g. Go 1.26), v1.61 won't BUILD. This script uses whatever golangci-lint is on $PATH and
#     reports its version so findings are interpretable. See validation_report.md for detail.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

RUN_E2E=1
[[ "${1:-}" == "--no-e2e" ]] && RUN_E2E=0

PASS=0; FAIL=0; SKIPPED=0
TMP="$(mktemp -d -t stagehand-validate-XXXXXX)"
trap 'rm -rf "$TMP"' EXIT

# color helpers (honored only on a TTY)
if [[ -t 1 && "${NO_COLOR:-}" == "" ]]; then
  G=$'\033[32m'; R=$'\033[31m'; Y=$'\033[33m'; B=$'\033[34m'; D=$'\033[1m'; N=$'\033[0m'
else
  G=""; R=""; Y=""; B=""; D=""; N=""
fi

ok()    { printf "  ${G}PASS${N}  %s\n" "$1"; PASS=$((PASS+1)); }
bad()   { printf "  ${R}FAIL${N}  %s\n" "$1"; FAIL=$((FAIL+1)); }
skip()  { printf "  ${Y}SKIP${N}  %s\n" "$1"; SKIPPED=$((SKIPPED+1)); }
hdr()   { printf "\n${D}${B}=== %s ===${N}\n" "$1"; }

# ---------------------------------------------------------------------------
# Pre-flight: ensure the toolchain + binaries are buildable
# ---------------------------------------------------------------------------
hdr "Phase 0 — pre-flight (toolchain + build)"
command -v go >/dev/null || { bad "go toolchain not on PATH"; exit 2; }
go version
if ! go build ./... ; then bad "go build ./... failed"; exit 2; else ok "go build ./..."; fi

# ---------------------------------------------------------------------------
# Phase 1 — Linting (golangci-lint, if present)
# ---------------------------------------------------------------------------
hdr "Phase 1 — linting"
if command -v golangci-lint >/dev/null 2>&1; then
  printf "  golangci-lint %s\n" "$(golangci-lint version 2>/dev/null | head -1)"
  if golangci-lint run --timeout=5m 2>"$TMP/lint.out"; then
    ok "golangci-lint clean"
  else
    bad "golangci-lint reported findings (config: .golangci.yml):"
    sed 's/^/      /' "$TMP/lint.out"
  fi
else
  skip "golangci-lint not installed (CI pins v1.61; install: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b \$(go env GOPATH)/bin v1.61.0)"
fi

# ---------------------------------------------------------------------------
# Phase 2 — Type / static analysis (go vet)
# ---------------------------------------------------------------------------
hdr "Phase 2 — type checking (go vet)"
if go vet ./... 2>"$TMP/vet.out"; then ok "go vet ./... clean"; else
  bad "go vet reported issues:"; sed 's/^/      /' "$TMP/vet.out"; fi

# ---------------------------------------------------------------------------
# Phase 3 — Style / formatting (gofmt)
# ---------------------------------------------------------------------------
hdr "Phase 3 — style checking (gofmt)"
gofmt -l . > "$TMP/fmt.out" 2>/dev/null
if [[ -s "$TMP/fmt.out" ]]; then
  bad "gofmt would reformat these files:"; sed 's/^/      /' "$TMP/fmt.out"
else
  ok "gofmt clean (no files need formatting)"
fi

# ---------------------------------------------------------------------------
# Phase 4 — Unit tests (race) + stub E2E suite
# ---------------------------------------------------------------------------
hdr "Phase 4 — unit + integration tests"
if go test -race -count=1 ./... 2>"$TMP/test.out"; then ok "go test -race ./... (all packages)"; else
  bad "go test -race ./... FAILED:"; sed 's/^/      /' "$TMP/test.out" | tail -40; fi

if go test -tags e2e -count=1 ./internal/e2e/... 2>"$TMP/e2e.out"; then
  ok "go test -tags e2e ./internal/e2e/... (stub scenarios)"
else
  bad "-tags e2e FAILED:"; sed 's/^/      /' "$TMP/e2e.out" | tail -40; fi

# ---------------------------------------------------------------------------
# Phase 5 — Coverage gate (PRD §20.3: >=85% on the 4 core packages)
# ---------------------------------------------------------------------------
hdr "Phase 5 — coverage gate (PRD §20.3, >=85% core pkgs)"
if make coverage-gate >/dev/null 2>"$TMP/cov.out"; then ok "make coverage-gate PASS"; else
  bad "coverage-gate FAILED:"; sed 's/^/      /' "$TMP/cov.out" | tail -20; fi

# ---------------------------------------------------------------------------
# Phase 6 — Vulnerability scan (govulncheck, CI runs this)
# ---------------------------------------------------------------------------
hdr "Phase 6 — vulnerability scan (govulncheck)"
if command -v govulncheck >/dev/null 2>&1; then
  if govulncheck ./... >/dev/null 2>"$TMP/vuln.out"; then ok "govulncheck: no vulnerabilities"; else
    bad "govulncheck reported vulnerabilities:"; sed 's/^/      /' "$TMP/vuln.out" | tail -30; fi
else
  skip "govulncheck not installed (install: go install golang.org/x/vuln/cmd/govulncheck@latest)"
fi

# ---------------------------------------------------------------------------
# Phase 7 — E2E product workflows (the documented user journeys)
# ---------------------------------------------------------------------------
if [[ $RUN_E2E -eq 0 ]]; then
  hdr "Phase 7 — E2E workflows"; skip "--no-e2e given; shell E2E skipped"
  printf "\n${D}RESULT: %d passed, %d failed, %d skipped${N}\n" "$PASS" "$FAIL" "$SKIPPED"
  [[ $FAIL -eq 0 ]] && exit 0 || exit 1
fi

hdr "Phase 7 — E2E product workflows (stub agent, throwaway repo)"

STUB="$TMP/stubagent"
BIN="$ROOT/bin/stagehand"
# Build both binaries (stagehand into ./bin via make, stub into tmp).
make build >/dev/null 2>&1 || { bad "make build failed (Phase 7 prerequisite)"; exit 2; }
go build -o "$STUB" ./cmd/stubagent >/dev/null 2>&1 || { bad "build stubagent failed"; exit 2; }

# Stub provider config (mirrors internal/e2e writeStubConfig).
cat > "$TMP/stub.toml" <<EOF
config_version = 3
[provider.stub]
command = "$STUB"
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
default_model = "stub"
EOF

CFG="$TMP/stub.toml"

# A fresh repo helper. Usage: mkrepo <dir>
mkrepo() {
  local d="$1"; rm -rf "$d"; git init -q "$d"
  git -C "$d" config user.name "Validate Bot"
  git -C "$d" config user.email "validate@example.com"
  printf 'init\n' > "$d/readme.md"
  git -C "$d" add readme.md
  git -C "$d" commit -q -m "init"
}

# --- W1: stage -> stagehand -> single commit ----------------------------------
d="$TMP/w1"; mkrepo "$d"; printf 'func main(){}\n' > "$d/login.js"; git -C "$d" add login.js
if STAGEHAND_STUB_OUT='feat: add login flow' "$BIN" --config "$CFG" --no-color --provider stub >/dev/null 2>&1 \
   && [ "$(git -C "$d" log -1 --format='%s')" = "feat: add login flow" ] \
   && [ "$(git -C "$d" diff-tree --no-commit-id --name-only -r HEAD)" = "login.js" ]; then
  ok "W1 single commit (stage -> stagehand -> commit)"
else bad "W1 single commit"; fi

# --- W2: -a auto-stage-all (dirty tree, nothing staged) -----------------------
d="$TMP/w2"; mkrepo "$d"; printf 'x\n' > "$d/a.txt"; printf 'y\n' > "$d/b.txt"
if STAGEHAND_STUB_OUT='chore: update a and b' "$BIN" --config "$CFG" --no-color --provider stub -a >/dev/null 2>&1 \
   && [ "$(git -C "$d" log -1 --format='%s')" = "chore: update a and b" ] \
   && [ -z "$(git -C "$d" status --porcelain)" ]; then
  ok "W2 -a auto-stage-all (dirty tree -> single commit, clean status)"
else bad "W2 -a auto-stage-all"; fi

# --- W3: --dry-run (generate, NO commit) --------------------------------------
d="$TMP/w3"; mkrepo "$d"; printf 'z\n' > "$d/c.txt"; git -C "$d" add c.txt
prev="$(git -C "$d" rev-parse HEAD)"
if STAGEHAND_STUB_OUT='feat: dry run' "$BIN" --config "$CFG" --no-color --provider stub --dry-run >/dev/null 2>&1 \
   && [ "$(git -C "$d" rev-parse HEAD)" = "$prev" ] \
   && grep -q '^A  c.txt$' <(git -C "$d" status --porcelain); then
  ok "W3 --dry-run (HEAD unchanged, file still staged)"
else bad "W3 --dry-run"; fi

# --- W4: --single (force single, dirty tree nothing staged) -------------------
d="$TMP/w4"; mkrepo "$d"; printf 'p\n' > "$d/d.txt"
if STAGEHAND_STUB_OUT='feat: single mode' "$BIN" --config "$CFG" --no-color --provider stub --single >/dev/null 2>&1 \
   && [ "$(git -C "$d" log -1 --format='%s')" = "feat: single mode" ]; then
  ok "W4 --single (forces single-commit path)"
else bad "W4 --single"; fi

# --- W4b: --commits 1 == --single ---------------------------------------------
d="$TMP/w4b"; mkrepo "$d"; printf 'p\n' > "$d/e.txt"
if STAGEHAND_STUB_OUT='feat: commits one' "$BIN" --config "$CFG" --no-color --provider stub --commits 1 >/dev/null 2>&1 \
   && [ "$(git -C "$d" log -1 --format='%s')" = "feat: commits one" ]; then
  ok "W4b --commits 1 == --single"
else bad "W4b --commits 1"; fi

# --- W5: rescue (unparseable output -> exit 3, repo unchanged) ----------------
d="$TMP/w5"; mkrepo "$d"; printf 'w\n' > "$d/w.txt"; git -C "$d" add w.txt; prev="$(git -C "$d" rev-parse HEAD)"
STAGEHAND_STUB_OUT='' "$BIN" --config "$CFG" --no-color --provider stub >"$TMP/w5.out" 2>&1; rc=$?
if [ "$rc" -eq 3 ] && [ "$(git -C "$d" rev-parse HEAD)" = "$prev" ] \
   && grep -q 'git commit-tree' "$TMP/w5.out"; then
  ok "W5 rescue (exit 3, HEAD unchanged, recovery recipe printed)"
else bad "W5 rescue (exit=$rc, want 3)"; fi

# --- W5b: --dry-run on failure -> exit 1 (NOT 3, no recipe) -------------------
d="$TMP/w5b"; mkrepo "$d"; printf 'w\n' > "$d/w.txt"; git -C "$d" add w.txt; prev="$(git -C "$d" rev-parse HEAD)"
STAGEHAND_STUB_OUT='' "$BIN" --config "$CFG" --no-color --provider stub --dry-run >"$TMP/w5b.out" 2>&1; rc=$?
if [ "$rc" -eq 1 ] && [ "$(git -C "$d" rev-parse HEAD)" = "$prev" ] \
   && ! grep -q 'git commit-tree' "$TMP/w5b.out"; then
  ok "W5b --dry-run failure (exit 1, no recovery recipe)"
else bad "W5b --dry-run failure (exit=$rc, want 1)"; fi

# --- W6: dedupe retry (subject matches recent -> retry) -----------------------
d="$TMP/w6"; mkrepo "$d"; git -C "$d" commit -q --allow-empty -m "feat: duplicate subject"
printf 'feat: dup\nchore: fresh unique msg' > "$TMP/dedupe.script"
printf 'u\n' > "$d/u.txt"; git -C "$d" add u.txt
if STAGEHAND_STUB_SCRIPT="$TMP/dedupe.script" STAGEHAND_STUB_COUNTER="$TMP/dedupe.cnt" \
   "$BIN" --config "$CFG" --no-color --provider stub >/dev/null 2>&1 \
   && [ "$(git -C "$d" log -1 --format='%s')" = "chore: fresh unique msg" ]; then
  ok "W6 dedupe retry loop (duplicate subject rejected, retried)"
else bad "W6 dedupe retry loop"; fi

# --- W7: fail-fast — uninstalled provider (exit 1, NO snapshot) ---------------
cat > "$TMP/missing.toml" <<EOF
config_version = 3
[provider.nope]
command = "/nonexistent/nope-agent"
prompt_delivery = "stdin"
output = "raw"
default_model = "x"
EOF
d="$TMP/w7"; mkrepo "$d"; printf 'z\n' > "$d/z.txt"; git -C "$d" add z.txt; prev="$(git -C "$d" rev-parse HEAD)"
STAGEHAND_STUB_OUT='x' "$BIN" --config "$TMP/missing.toml" --no-color --provider nope >"$TMP/w7.out" 2>&1; rc=$?
if [ "$rc" -eq 1 ] && [ "$(git -C "$d" rev-parse HEAD)" = "$prev" ]; then
  ok "W7 fail-fast uninstalled provider (exit 1, HEAD unchanged)"
else bad "W7 fail-fast uninstalled provider (exit=$rc)"; fi

# --- W7b: fail-fast — missing STAGEHAND_CONFIG (exit 1, no silent fallback) ----
d="$TMP/w7b"; mkrepo "$d"; printf 'z\n' > "$d/k.txt"; git -C "$d" add k.txt
STAGEHAND_CONFIG="$TMP/DOES_NOT_EXIST.toml" STAGEHAND_STUB_OUT='x' \
  "$BIN" --no-color >"$TMP/w7b.out" 2>&1; rc=$?
if [ "$rc" -eq 1 ] && grep -qi 'not found' "$TMP/w7b.out"; then
  ok "W7b fail-fast missing STAGEHAND_CONFIG (exit 1, no silent fallback)"
else bad "W7b fail-fast missing STAGEHAND_CONFIG (exit=$rc)"; fi

# --- W8: FR-R5b multi-backend bare model -> hard error exit 1 -----------------
cat > "$TMP/multi.toml" <<EOF
config_version = 3
[provider.stub]
command = "$STUB"
prompt_delivery = "stdin"
output = "raw"
default_model = "stub"
[provider.testmulti]
command = "$STUB"
prompt_delivery = "stdin"
output = "raw"
model_flag = "--model"
provider_flag = "--provider"
default_model = "x"
EOF
d="$TMP/w8"; mkrepo "$d"; printf 'm\n' > "$d/m.txt"; git -C "$d" add m.txt; prev="$(git -C "$d" rev-parse HEAD)"
STAGEHAND_STUB_OUT='feat: x' "$BIN" --config "$TMP/multi.toml" --no-color --provider testmulti --model bare >"$TMP/w8.out" 2>&1; rc=$?
if [ "$rc" -eq 1 ] && grep -q 'must be inference/model' "$TMP/w8.out" && [ "$(git -C "$d" rev-parse HEAD)" = "$prev" ]; then
  ok "W8 FR-R5b bare-model hard error (exit 1, repo unchanged)"
else bad "W8 FR-R5b bare-model (exit=$rc)"; fi

# --- W9: binary file in staged changes (graceful filtering) -------------------
d="$TMP/w9"; mkrepo "$d"
printf '\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR' > "$d/image.png"
printf 'code change\n' > "$d/app.go"
git -C "$d" add image.png app.go
if STAGEHAND_STUB_OUT='feat: add image and code' "$BIN" --config "$CFG" --no-color --provider stub >/dev/null 2>&1 \
   && [ "$(git -C "$d" log -1 --format='%s')" = "feat: add image and code" ] \
   && { git -C "$d" diff-tree --no-commit-id --name-only -r HEAD | grep -qx 'image.png'; } \
   && { git -C "$d" diff-tree --no-commit-id --name-only -r HEAD | grep -qx 'app.go'; }; then
  ok "W9 binary file filtering (both files committed, no crash)"
else bad "W9 binary file filtering"; fi

# --- W10: decompose planner SINGLE shortcut (2 files, planner single:true) ----
d="$TMP/w10"; mkrepo "$d"; printf 'aaa\n' > "$d/fa.txt"; printf 'bbb\n' > "$d/fb.txt"
PLANJSON='{"count":1,"single":true,"commits":[{"title":"add a and b","description":"a+b"}],"message":"feat: add a and b"}'
if STAGEHAND_STUB_OUT="$PLANJSON" "$BIN" --config "$CFG" --no-color --provider stub >/dev/null 2>&1 \
   && [ "$(git -C "$d" log -1 --format='%s')" = "feat: add a and b" ] \
   && [ "$(git -C "$d" rev-list --count HEAD)" -eq 2 ] \
   && [ -z "$(git -C "$d" status --porcelain)" ]; then
  ok "W10 decompose planner single-shortcut (planner message verbatim, both files)"
else bad "W10 decompose planner single-shortcut"; fi

# --- W11: config init (FR-B1: uncommented reasoning = "off" in [defaults]) ----
initcfg="$TMP/init.toml"; rm -f "$initcfg"
if "$BIN" --config "$initcfg" config init >/dev/null 2>&1 \
   && grep -Eq '^[[:space:]]*reasoning[[:space:]]*=[[:space:]]*"off"' "$initcfg" \
   && grep -Eq '^[[:space:]]*provider[[:space:]]*=' "$initcfg"; then
  ok "W11 config init (FR-B1: uncommented reasoning=\"off\" in [defaults])"
else bad "W11 config init"; fi

# --- W12: config upgrade (v1 -> current schema) -------------------------------
upcfg="$TMP/upgrade.toml"
cat > "$upcfg" <<'EOF'
provider = "stub"
model = "stub"
timeout = "120s"
EOF
if "$BIN" --config "$upcfg" config upgrade >/dev/null 2>&1 \
   && grep -Eq '^config_version[[:space:]]*=' "$upcfg"; then
  ok "W12 config upgrade (adds config_version)"
else bad "W12 config upgrade"; fi

# --- W13: providers list + show -----------------------------------------------
if "$BIN" providers list >/dev/null 2>&1 && "$BIN" providers show pi >/dev/null 2>&1; then
  ok "W13 providers list / show"
else bad "W13 providers list / show"; fi

# --- W14: --version + --help --------------------------------------------------
"$BIN" --version >/dev/null 2>&1 && vrc=$? || vrc=$?
"$BIN" --help >/dev/null 2>&1 && hrc=$? || hrc=$?
if [ "$vrc" -eq 0 ] && [ "$hrc" -eq 0 ]; then ok "W14 --version / --help"; else bad "W14 --version / --help"; fi

# ---------------------------------------------------------------------------
# Result
# ---------------------------------------------------------------------------
printf "\n${D}RESULT: %d passed, %d failed, %d skipped${N}\n" "$PASS" "$FAIL" "$SKIPPED"
[[ $FAIL -eq 0 ]] && { printf "${G}VALIDATION PASSED${N}\n"; exit 0; }
printf "${R}VALIDATION FAILED${N}\n"; exit 1
