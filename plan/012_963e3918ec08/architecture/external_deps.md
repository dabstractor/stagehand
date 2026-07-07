# External Dependencies

## Go Dependencies (go.mod)

| Module | Version | Rename impact |
|--------|---------|---------------|
| `github.com/spf13/cobra` | v1.10.2 | None — external, no stagehand refs |
| `github.com/spf13/pflag` | v1.0.10 | None — external |
| `github.com/pelletier/go-toml/v2` | v2.4.2 | None — external |
| `gopkg.in/yaml.v3` | v3.0.1 | None — external |
| `github.com/inconshreveable/mousetrap` | v1.1.0 | Indirect (cobra dep), none |

## System Dependencies

| Dependency | Required | Rename impact |
|-----------|----------|---------------|
| Go toolchain | ≥1.22 | Module path validity only |
| git binary | ≥2.20 | Runtime shells — no rename impact |

## No External Importers

The module `github.com/dustin/stagehand` is a private project with no external consumers. No `go.mod` anywhere else imports it. The rename is fully self-contained — no coordinated updates needed outside this repository.

## Distribution Channels (all reference the project name)

| Channel | Current reference | Post-rename |
|---------|------------------|-------------|
| Homebrew tap | `dustin/homebrew-tap` formula `stagehand` | formula `stagecoach` |
| Scoop bucket | `dustin/scoop-bucket` manifest `stagehand` | manifest `stagecoach` |
| AUR | `stagehand-bin` | `stagecoach-bin` |
| GitHub Releases | `github.com/dustin/stagehand` | `github.com/dabstractor/stagecoach` (matches actual remote) |
| go install | `github.com/dustin/stagehand/cmd/stagehand@latest` | path TBD (module path + cmd dir name) |

**Note:** The GitHub remote is `dabstractor/stagecoach`, but goreleaser/README reference `dustin/stagehand`. The canonical GitHub path must be reconciled. For this rename, use `dabstractor/stagecoach` to match the actual remote, or `dustin/stagecoach` if the repo will be moved. The implementing agent should verify the correct GitHub org/name.
