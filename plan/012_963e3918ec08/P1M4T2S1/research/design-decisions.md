# P1.M4.T2.S1 — Rename providers/*.toml comment references: Findings

> Companion to `../PRP.md`. A focused mechanical rename (1 point). This file captures the verified facts
> that make the PRP deterministic.

## F1 — 18 references, ALL in comments; ZERO functional values touched

Grep over `providers/*.toml` finds 18 `stagehand` matches across the 8 files. A rigorous check — strip
every `#`-comment line, then grep the remainder for `stagehand` — returns **empty**: every match is on a
comment line. No TOML key, value, flag, command, or model token contains `stagehand`. So the blanket
`s/stagehand/stagecoach/g` cannot change a functional value. (There are **no** `Stagehand`/`STAGEHAND`
matches either — only lowercase `stagehand` — so the `s/Stagehand/Stagecoach/g` arm of the contract sed is
a harmless no-op; keep it for symmetry with the other rename PRPs.)

## F2 — The reference breakdown (boilerplate header in every file; pi.toml has a unique line)

- **Lines 9 + 16 — the shared "WHAT THIS FILE IS / HOW TO USE IT" header** (present in all 8 files):
  - Line 9: `#   the Go binary. (The config loader reads .stagehand.toml, not this directory.)` → 1 ref.
  - Line 16: `#       # ~/.config/stagehand/config.toml  (or a repo-local .stagehand.toml)` → **2 refs**
    (`stagehand/config.toml` AND `.stagehand.toml`).
- **pi.toml line 112 — the UNIQUE non-boilerplate ref**: `# off has no entry ⇒ graceful no-op (FR-R6);
  minimal/xhigh have no stagehand level.` → `no stagecoach level`.
- Per-file line counts (from `grep -rc`): agy=3, claude=2, codex=2, cursor=2, gemini=2, opencode=2, pi=3,
  qwen-code=2. Total = 18 lines (line 16 carries two occurrences, so ~19 substitutions; the blanket sed
  handles both per-line in one pass).

The blanket `sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g' providers/*.toml` covers every
occurrence regardless of the per-line counts.

## F3 — The comment updates are ACCURATE post-rename (not just cosmetic)

The boilerplate comments reference the config loader's REAL paths:
- `.stagehand.toml` → `.stagecoach.toml` — the repo-local config file, renamed in **P1.M2.T2.S2** (Complete).
  Verified: `internal/config/file.go:125` reads `./.stagecoach.toml`; `internal/config/bootstrap.go:244,246`.
- `~/.config/stagehand/config.toml` → `~/.config/stagecoach/config.toml` — the global config path, renamed
  in **P1.M2.T2.S1** (Complete). Verified: `internal/config/file.go:92-94`.

So leaving these comments as `stagehand` would make the documentation **wrong** (pointing users at config
paths the renamed binary no longer reads). The rename makes them correct again.

## F4 — No conflict with the parallel P1.M4.T1.S2 (docs/*.md)

The previous PRP (P1.M4.T1.S2, "Layer 5.2") modifies ONLY `docs/{cli,configuration,how-it-works,providers,
README}.md`. It does NOT touch `providers/*.toml`. The two tasks are disjoint file sets — no merge conflict.
(rename_surface_map Layer 5.2 = docs/; Layer 5.3 = providers/*.toml — this task.)

## F5 — These TOMLs are REFERENCE docs, NOT loaded at runtime

Per the boilerplate header itself ("It is NOT loaded at runtime — built-ins are compiled into the Go
binary"), `providers/*.toml` are human-readable REFERENCE manifests mirroring `internal/provider/builtin.go`
byte-for-byte (modulo comments). They are NOT parsed at runtime. So a comment-only rename has ZERO runtime
effect — no test reads these files' comments (the manifest tests exercise `builtin.go`, not the TOML). The
validation is purely textual (grep + a diff showing only `#` lines changed).

## D1–D3 — Decision summary

- **D1 (blanket case-variant sed):** `sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g' providers/*.toml`
  — the contract's exact command. Covers all occurrences in one pass. The `Stagehand` arm is a no-op but
  kept for symmetry with the sibling rename PRPs.
- **D2 (verify comments-only):** after the sed, `git diff providers/*.toml` shows ONLY `#`-comment lines
  changed. Pre-checked (F1): no functional value matches, so the sed is comment-only by construction.
- **D3 (zero-residue grep):** `grep -rn 'stagehand\|Stagehand\|STAGEHAND' providers/*.toml` returns empty
  (exit 1) after the rename — the success criterion.

## SCOPE BOUNDARY (owned by siblings — do NOT touch)
- `internal/provider/builtin.go` (compiled-in manifests — Go source, renamed in M1.T2). docs/*.md
  (P1.M4.T1.S2, Layer 5.2 — parallel). FUTURE_SPEC.md (P1.M4.T2.S2). plan/ artifacts (P1.M5.T1).
  PRD.md / tasks.json / prd_snapshot.md (orchestrator-owned, READ-ONLY).
- Do NOT touch any functional TOML value (name/command/flags/models/`[reasoning_levels]`/`[env]`). The sed
  is comment-only by construction (F1).
