# P1.M4.T3.S1 — External Research (config upgrade: TOML preservation)

Stagecoach uses `github.com/pelletier/go-toml/v2` (already in go.mod). The central design question for
`config upgrade` is whether to **round-trip through TOML** (unmarshal → mutate → marshal) or do
**minimal textual edits**. The contract (FR-B5) decides it.

## 1. go-toml/v2 marshal DROPS comments and reorders — unsuitable for "preserve structure"

- `toml.Unmarshal(data, &v)` parses into a struct/map but **discards all comments and formatting** — the
  AST does not retain `# ...` lines or blank-line grouping.
- `toml.NewEncoder(w).Encode(v)` re-emits in **go-toml's canonical layout** (tables grouped, keys sorted
  within tables, go-toml's own quoting/spacing rules). It does NOT reproduce the user's original layout.
- Reference: https://github.com/pelletier/go-toml/v2 (Encoder "marshals … keys are written in a
  deterministic order"; no comment preservation). https://pkg.go.dev/github.com/pelletier/go-toml/v2#Encoder.Encode

⇒ **Conclusion: a round-trip would rewrite the ENTIRE file** — stripping every comment, reordering
sections, changing quoting — which directly violates the FR-B5 contract:
> "preserving user values for keys that still exist, comments out removed/renamed keys with a note …
> leave all other content unchanged."

⇒ **The upgrade MUST be a minimal textual transform**: read the file as text, set/add ONLY the
top-level `config_version` line, leave every other byte identical. (We still `toml.Unmarshal` once into
`map[string]any` purely as a *validity gate* — refuse to mangle an unparseable file — but never marshal back.)

## 2. TOML: top-level keys must precede `[table]` headers

Per the TOML spec (https://toml.io/en/v1.0.0#keys), bare `key = value` pairs at the start of a document
are **root-level (top-level)** keys. Once a `[table]` header appears, subsequent keys belong to that table.
So `config_version` (root metadata, §9.17 FR-B4 — `toml:"config_version"` on the root `fileConfig`
struct) can ONLY appear before the first `[section]`.

**Implication for the textual transform:** scan for `config_version = N` only in the **top-level region**
(lines before the first `[table]` header). A `config_version` seen AFTER a `[table]` is a different key
(e.g. it'd decode into that table) — not the schema version. Breaking the scan at the first `[table]`
header avoids a false match and avoids creating a duplicate root key.

## 3. go-toml/v2 unmarshal is non-strict by default (safe validity gate)

- `toml.Unmarshal(data, &map[string]any{})` succeeds on any syntactically-valid TOML and **ignores keys
  not present in the target** (no `Strict()`). So a v1 config containing only `[defaults]` — or unknown
  future keys — parses cleanly. The only failure mode is a genuine *syntax* error (unclosed string,
  bad table header, trailing junk), which is exactly what we want to reject before editing.
- Reference: https://pkg.go.dev/github.com/pelletier/go-toml/v2#Unmarshal (use `Strict(true)` to error on
  unknown keys; we deliberately do NOT — a v1 file is valid even if it omits v2 keys).

⇒ Use `map[string]any` (not the typed `fileConfig`) for the gate, so the gate never rejects a merely-
incomplete config; it only rejects a malformed one. (Reading the existing version is done textually, not
from this map, so the map is purely a syntax check.)

## 4. Idempotency is a property of the textual transform, not a special case

The transform is: "ensure the top-level `config_version` line equals N, changing/inserting as little as
possible." Run once on a v1 file (no `config_version`) → inserts the line. Run again → finds `config_version
= N`, value already equals N → returns the content **byte-identical** with `changed=false` → the command
prints "already up to date" and does NOT rewrite. Hence idempotent by construction; no separate handling.
