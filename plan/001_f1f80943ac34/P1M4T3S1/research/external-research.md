# P1.M4.T3.S1 — External Research (UI / Color / TTY / NO_COLOR)

Authoritative references for the `internal/ui` output helpers. All links verified relevant to the
implementation. Stagecoach is **stdlib-only** (see `internal/provider/procgroup_windows.go` — it avoids
`golang.org/x/sys` via `syscall.NewLazyDLL`); we do NOT pull in `golang.org/x/term` for TTY detection.

## 1. The NO_COLOR convention — https://no-color.org

> "Command-line software which adds ANSI color to its output by default should check for the presence
> of a `NO_COLOR` environment variable that, when present **and not an empty string** (regardless of its
> value), prevents the addition of ANSI color."

**Implementation rule (matches codebase idiom):**
```go
v, ok := os.LookupEnv("NO_COLOR")
return ok && v != ""   // present AND non-empty → disable color
```
This is byte-for-byte the SAME idiom `internal/config/load.go` uses for `STAGECOACH_NO_COLOR`
(line 112). Using it for `NO_COLOR` keeps the two consistent. `NO_COLOR=""` (set without a value) does
NOT disable; `NO_COLOR=1` / `NO_COLOR= ` (space) / any non-empty value DOES.

**Precedence vs `--no-color` / `STAGECOACH_NO_COLOR`:** all three are "disable" signals that OR
together. There is no "force color" override in v1 (the spec's `CLICOLOR_FORCE` is out of scope). Final
color gate:
```
color = isTTY(stdout) AND NOT cfg.NoColor AND NOT noColorEnvSet()
```
- `cfg.NoColor` already folds in `--no-color` (Layer 7) + `STAGECOACH_NO_COLOR` (Layer 5) via
  `internal/config/load.go` → the UI layer only adds `isTTY` + the bare `NO_COLOR` env on top.

## 2. TTY detection WITHOUT a dependency — stdlib `os.FileStat`

`golang.org/x/term.IsTerminal` is the "fancy" option but needs a new dependency. The stdlib heuristic
(used by many CLIs that want zero deps) is **character-device detection**:

```go
func IsTerminal(f *os.File) bool {
    stat, err := f.Stat()
    if err != nil {
        return false
    }
    return (stat.Mode() & os.ModeCharDevice) != 0
}
```

**Why it works:** a real terminal (`/dev/tty`, a pty) is a character device; a pipe, a file redirect,
or `cmd | tee` is a regular file or pipe (NOT `ModeCharDevice`). This is exactly the discrimination
FR51 needs ("color when stdout is a TTY").

**Gotcha (documented, accepted):** `ModeCharDevice` is a heuristic, not a true `isatty` ioctl
(`TCGETS` on Linux). Rare false positives (a char device that is not a terminal) could enable color.
This is acceptable because (a) the `--no-color` / `NO_COLOR` overrides remain authoritative, and (b)
the project's stated philosophy is zero new deps (`procgroup_windows.go` proves it). If precision ever
matters, swap the body for `golang.org/x/term` later — the `IsTerminal` signature is stable.

**Reference:** https://pkg.go.dev/os#FileStat (ModeCharDevice), https://pkg.go.dev/os#ModeCharDevice

## 3. ANSI SGR color codes

| Color | Code   | Const    | Use (Stagecoach)          |
|-------|--------|----------|--------------------------|
| red   | `\x1b[31m` | `ansiRed`    | Error / failure notices  |
| green | `\x1b[32m` | `ansiGreen`  | Success / "Created"      |
| yellow| `\x1b[33m` | `ansiYellow` | Progress / warnings      |
| reset | `\x1b[0m`  | `ansiReset`  | always appended after color |

Always emit `<code><text><reset>`. Never emit a bare leading code without a trailing reset (leaks
color into subsequent lines / piped consumers). When color is OFF, helpers return the string unchanged
(no codes at all — keeps `git commit -F <(stagecoach --dry-run)` clean).

**Reference:** https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_(Select_Graphic_Rendition)_parameters

## 4. The "↳" progress glyph — PRD Appendix B

`↳` = **U+21B3** "DOWNWARDS ARROW WITH TIP RIGHTWARDS" (verified: `python3 -c "print('U+%04X' % ord('↳'))"`
→ `U+21B3`). Used as the prefix for every progress line in Appendix B.1–B.4:
- `↳ Snapshotting 2 staged files…  (tree 9f3a1c…)`
- `↳ Generating with pi (glm-5.2)…`
- `↳ Created abc1234  feat(auth): accept SAML tokens for enterprise login`
- `↳ Attempt 1: subject "..." matches an existing commit — retrying.` (verbose — P1.M4.T3.S2)

The ellipsis `…` is U+2026 (matches PRD verbatim — do NOT use three ASCII dots).

## 5. Stream discipline (FR51 / §15.5) — the governing constraint

- **stdout = the RESULT, always PLAIN (zero ANSI).** Why: `stagecoach --dry-run --no-color | tee
  /tmp/msg.txt` and `git commit -F <(stagecoach --dry-run)` (§15.5) require stdout to be a clean
  message with no control codes. The existing `default_action_test.go` asserts `stdout == "feat: dry
  run"` (EXACT, post-TrimSpace) and `Contains(stdout, "] feat: add login")` — colorizing stdout
  would break the exact-equality test AND pollute pipes.
- **stderr = progress + notices + diagnostics, MAY be colored.** FR51 ("progress messages go to
  stderr so stdout stays clean for piping"). `Contains(stderr, "Nothing staged — staging all changes
  (2 files).")` still passes when ANSI wraps the plaintext (substring match survives wrapping).

⇒ Design consequence: **only `os.Stderr`-bound output ever carries ANSI.** `Success`/`Progress`/
`Error` helpers write to the STDERR writer. The data surface (`printCommitReport`,
`printDryRunMessage`) stays plain on stdout.
