# docs/ Stale-Claim Matrix — Cross-Reference of the Five Fixes

Research artifact for P1.M6.T1.S2. Maps each fix → the exact stale strings in `docs/`,
with the authoritative post-fix wording (sourced from the ALREADY-UPDATED `README.md`
(root) and `providers/{pi,claude}.toml`, which serve as the canonical references).

All five fixes are CONFIRMED APPLIED in code; `go build ./...` and `go test ./...` pass.

## Fix → docs impact summary

| Fix | docs/cli.md | docs/configuration.md | docs/providers.md | docs/how-it-works.md | docs/README.md |
|-----|-------------|----------------------|--------------------|----------------------|----------------|
| 1 (provider/sub-provider) | optional clarify | optional clarify | optional clarify | — | — |
| 2 (stager toolset) | — | — | STALE (L94 + safety layers) | STALE (L115) | — |
| 3 (post-arbiter output) | — | — | — | — (no stale claim) | — |
| 4 (config override) | STALE (L46, 76, 111) | STALE (L31, 67) | — | — | — |
| 5 (pi bootstrap) | STALE (L76) | STALE (L38) | STALE (L109) | — | — |

## MUST-FIX stale strings (exact locations, 2026-07-01)

### docs/cli.md
- **L46** (`--config` flag): "honored by every command — including the default commit action ...
  (not just the `providers`/`config` subcommands)." → Issue 4. WRONG: after fix, `config
  init/upgrade/path` DO honor `--config`. Replace with README wording: "...including the
  default commit action **and the `config init`, `config path`, and `config upgrade`
  subcommands** (e.g. `stagehand --config X config upgrade` upgrades file `X`, and
  `config path` prints the resolved path)."
- **L76** (`config init`): "Bootstrap a populated, working config to the **global config path**
  ... that provider's per-role model defaults UNCOMMENTED so the tool works immediately." →
  Issue 4 (path) + Issue 5 (pi models). Path is now override-aware; for **pi** (default) per-role
  models are left EMPTY so pi picks its own backend model; set `[provider.pi] default_provider`
  to pin a backend.
- **L111–116** (`config path`): "Print the resolved **global** config path." → Issue 4. Now
  prints the override-aware path (`ResolveConfigPath`).

### docs/configuration.md
- **L31**: "Use `stagehand config path` to print the resolved **global** path." → Issue 4.
- **L38** (Bootstrap step 2): "Writes ... that provider's per-role model defaults UNCOMMENTED
  (from the FR-D4 table)." → Issue 5 (pi models blanked).
- **L67** (NOTE): "honored by every command — including the default commit action" → Issue 4
  (now includes config subcommands).

### docs/providers.md
- **L94** (stager safety Layer 1): "scopes tools to staging (claude: git/read/edit allowlist
  via `--allowed-tools`; pi: all tools on, chrome stripped)." → Issue 2. claude is now a
  STRUCTURAL staging-only git allowlist `Bash(git add:*,git apply:*,git status:*,git diff:*)`
  + Read+Edit (commit/amend/push/reset unreachable); pi is NOT flag-scoped and is
  INSTRUCTIONAL (§17.6 prompt) + BEST-EFFORT HEAD-movement guard (not structurally constrained).
- **L94 Layers 2–3**: ref-mutation monopoly + stager task prompt → note pi relies on the HEAD
  guard; claude is structurally constrained. (Mirrors pi.toml/claude.toml honest comments.)
- **L109**: "The compiled-in per-provider table ... is the source of truth for the config
  bootstrap (`config init`, P1.M4.T2)." → Issue 5. The table is the compiled-in FR-D4 defaults;
  the bootstrap uses them EXCEPT for pi, whose models are blanked (pi needs `default_provider`
  to route `gpt-5.4*`). The pi table row (L113) is still accurate as compiled-in defaults.

### docs/how-it-works.md
- **L115** (Multi-commit "Safety"): "The stager is the ONE role that touches the index
  (scoped strictly to `git add`) ..." → Issue 2. "scoped strictly to `git add`" is only
  structurally true for claude. For pi it is instructional + HEAD guard. Update to distinguish
  the two stager-capable providers honestly (mirror README "Multi-commit decomposition" prose).

## OPTIONAL clarifications (consistency, not stale)
- Issue 1: providers.md / cli.md may add a note that pi's `--provider` flag carries the
  SUB-PROVIDER (from `[provider.pi] default_provider`), NOT the manifest name (mirrors README
  NOTE in "Configure your agent"). No doc currently shows `pi --provider pi`, so not strictly stale.
- Issue 3: no doc claims stale SHAs are printed; the fix makes the documented success report
  accurate. No change required; verify only.
- how-it-works.md L129 (bare-role §18.1 invariant): "No provider mutates the repository" is
  about the BARE roles (planner/message/arbiter) — pre-existing framing, not a regression.
  Optionally clarify the stager (tooled) is the documented index-mutating exception.

## Authoritative post-fix wording sources (already updated, do NOT reword differently)
- `README.md` (root) — "Configure your agent" NOTE (pi sub-provider), "Multi-commit
  decomposition" (stager safety: claude allowlist / pi instructional + HEAD guard), and the
  `--config` NOTE (honored by config subcommands). Mirror these exactly in docs/.
- `providers/pi.toml` — "SAFETY MODEL — HONEST" block.
- `providers/claude.toml` — tooled_flags staging-only allowlist block.
