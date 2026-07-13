# Critical Findings — v2.9 Chrome-disable

## Finding 1: No code changes needed beyond doc-comments

The provider manifests in `internal/provider/builtin.go` already set every chrome-disable flag
that each agent CLI exposes. The only code-touching work is:

1. **CHROME-DISABLE notes** in each of the 7 provider doc-comment blocks (FR-C5)
2. **Test assertions** pinning the chrome-disable contract (M1.T2)
3. **Mirror notes** in `providers/*.toml` reference headers (consistency)

No new bare_flags entries are expected. Verification may surface a small number of missing flags for
the read-only-constrained providers, but research shows they mostly expose no per-surface chrome
switches.

## Finding 2: pi MCP gap is the one real "tracked limitation"

pi has no `--no-mcp` flag (only `--mcp-config <path>`). `--no-tools` suppresses MCP tool *use* but
MCP servers configured in the user's settings may still be discovered/connected at startup. This is
documented in external_deps.md §pi and must appear in pi's CHROME-DISABLE note as a tracked
limitation per FR-C3.

## Finding 3: Test approach — manifest assertions, no agent calls

Chrome-disable tests are cheap table-driven assertions against the `Manifest` struct:

- Assert pi `BareFlags` contains `--no-extensions`, `--no-skills`, `--no-prompt-templates`,
  `--no-context-files` (regression guard)
- Assert claude `BareFlags` contains `--setting-sources` and `--tools` (regression guard)
- Assert each read-only-constrained provider's constraint flag is present
- Optionally: parse CHROME-DISABLE note text for claimed flags and assert presence in bare_flags

No real agent invocation is needed. All tests are pure struct assertions.

## Finding 4: Documentation sync — Mode A + Mode B

Per §5 of the SOW:

- **Mode A (ride with work):** CHROME-DISABLE notes in `builtin.go` are the per-provider
  documentation artifact — they live alongside the code they describe (M1.T1)
- **Mode B (final task):** `docs/providers.md` table column + asymmetry section extension,
  `docs/how-it-works.md` safety paragraph extension, `docs/README.md` capability index entry,
  top-level `README.md` brief mention (M2.T1)

The Mode B task depends on M1.T1 (it records the verified manifest state).

## Finding 5: Existing test structure to follow

`internal/provider/builtin_test.go` uses:
- `assertStr(t, "FieldName", got, want)` helper
- `assertNilStr(t, "FieldName", ptr)` helper
- Table-driven test cases with `reflect.DeepEqual` for slices
- Provider-specific test functions: `TestBuiltinManifests_PiFields`, `_ClaudeFields`, etc.
- `renderArgs(m, provider, model, sys)` for rendered-command checks

New chrome-disable tests should follow this exact pattern — add assertions to the existing
per-provider field tests (or add a focused new test function like `TestBuiltinManifests_ChromeDisable`).

## Finding 6: The §12.7.1 asymmetry table extension

The existing "## Tools-disable asymmetry" section in `docs/providers.md` has two bullets
(explicit switch vs read-only constraint). The v2.9 extension adds a conceptual 4th consequence
bullet: "Chrome is a separate axis." This is documentation, not mechanism change.

The 7-provider table in `docs/providers.md` needs a new "Chrome-disable" column recording the
chrome outcome in one short phrase per provider.
