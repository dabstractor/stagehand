# P1.M5.T1.S1 — Research Findings
## Defense-in-depth agent→provider textual remap in loadTOML (bugfix Issue 5)

---

## 0. Task contract (verbatim from item_description)

Goal: a v2 config file using the abandoned intermediate `agent`/`[agent.*]` terminology loads with its
provider block preserved **in memory** (`cfg.Provider` / `cfg.Providers` populated), WITHOUT requiring the
user to run `config upgrade` first.

The fix: add `remapAgentTerminology(data []byte) []byte` to `internal/config/file.go` and call it on the
raw TOML bytes in `loadTOML` BEFORE `toml.Unmarshal(data, &fc)`. Two textual transforms:
- (a) `[agent.` → `[provider.` (table headers: `[agent.pi]` → `[provider.pi]`).
- (b) line-oriented: a line matching `^\s*agent\s*=` has its key `agent` rewritten to `provider`
      (preserves indent + `=` + value; does NOT touch `agent` inside values/comments/headers).

Must be IDEMPOTENT (a no-op on an already-provider file). DOCS: update the `migrateV2ToV3` doc comment
(migrate.go) — the agent→provider remap is no longer a pure in-memory no-op; it's handled textually
upstream in loadTOML.

---

## 1. The decode seam — `internal/config/file.go` (the file to EDIT)

`loadTOML(path)` (file.go:124-150):
```go
data, err := os.ReadFile(path)
if err != nil {
    if os.IsNotExist(err) { return nil, nil }
    return nil, fmt.Errorf("read config %s: %w", path, err)
}
// <<< INSERT data = remapAgentTerminology(data) HERE, before the typed decode >>>
var fc fileConfig
if err := toml.Unmarshal(data, &fc); err != nil {
    return nil, fmt.Errorf("parse config %s: %w", path, err)
}
```

`fileConfig` (file.go:30-37) — fields: `ConfigVersion, Defaults, Generation, Role, Provider`. **NO `Agent`
field.** So `[agent.*]` tables and a `[defaults] agent =` key are SILENTLY DROPPED by go-toml/v2 today
(confirmed by the bug probe: `cfg.Provider == ""`, no `cfg.Providers["pi"]`).

**Imports in file.go today** (file.go:3-10): `fmt, io, os, path/filepath, time` +
`github.com/pelletier/go-toml/v2`. **`strings` AND `regexp` are NOT imported** → both must be ADDED.

**Trace proving the fix populates cfg** (after remap, the typed decode works):
- `[defaults]\nagent = "pi"` → remap → `[defaults]\nprovider = "pi"` → `fc.Defaults.Provider = "pi"`
  (fileDefaults has `Provider string toml:"provider"`) → `materialize` sets `c.Provider = "pi"`. ✓
- `[agent.pi]\ndefault_model = "glm-5.2"` → remap → `[provider.pi]\n...` → `fc.Provider["pi"]` populated
  → `materialize`: `c.Providers = fc.Provider` (whole-map copy) → `cfg.Providers["pi"]` non-empty. ✓

---

## 2. The proven on-disk precedent — mirror it (`internal/cmd/config.go`, READ ONLY)

The codebase ALREADY has a textual `[agent.*]`→`[provider.*]` remap for the ON-DISK `config upgrade`
path. `internal/cmd/config.go`:
```go
// agentHeaderRe captures the name in an `[agent.<name>]` table header (config.go:125-127).
var agentHeaderRe = regexp.MustCompile(`^\[agent\.(.+?)\]\s*$`)
// rewriteV2ToV3 pass 1 (config.go:~260): rename every `[agent.<name>]` header → `[provider.<name>]`.
```
→ This PROVES the textual-replace approach is the established house pattern for FR-B7. The in-memory
loadTOML remap is the IN-MEMORY twin (same FR-B7 "first map agent→provider" step, different domain: raw
bytes-before-decode vs on-disk lines-after-read). Mirror the regexp idiom; keep them independent (the
on-disk rewrite also folds default_provider, which loadTOML does NOT — that stays migrateV2ToV3's job).

The on-disk `kvStringRe = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_-]*)\s*=\s*"([^"]*)"`)`
(config.go:138-139) is the EXACT line-oriented key-match idiom the contract's `^\s*agent\s*=` maps to.

---

## 3. The two transforms — precise, idempotent, safe

### (a) Table headers: `[agent.` → `[provider.`
```go
s := strings.ReplaceAll(string(data), "[agent.", "[provider.")
```
Literal prefix replace. Idempotent: `[provider.` does not contain `[agent.`, so a re-run is a no-op.
Safe: `[agent.` as a substring only occurs in table headers in practice (a model value like
`"[agent.foo]"` is nonsensical; a comment `# [agent.pi]` → `# [provider.pi]` stays an inert comment).

### (b) Bare key: `agent =` → `provider =` (line-oriented, KEY NAME ONLY)
```go
var agentKeyRe = regexp.MustCompile(`(?m)^(\s*)agent(\s*=)`)
// ...
s = agentKeyRe.ReplaceAllString(s, "${1}provider${2}")
```
Multiline `(?m)` so `^` matches each line start. Captures (1) leading whitespace + (2) the ws+`=`;
rebuilds as `${1}provider${2}` — preserves indent and the `=`. Matches `agent = "pi"`, `agent="pi"`,
`\tagent = "pi"`. Does NOT match:
- `# agent = "pi"` (comment — starts with `#`, not `agent` after `^(\s*)`).
- `model = "agent"` (starts with `model`).
- `my_agent = "x"` (starts with `my_`).
- `[agent.pi]` (starts with `[` — handled by (a)).

Idempotent: after remap the key is `provider =`, which `^(\s*)agent(\s*=)` does not match.

**Schema safety**: in the v3 schema NO legitimate key is named `agent` anywhere (defaults keys:
provider/model/reasoning/timeout/auto_stage_all/verbose; role keys: provider/model/reasoning;
provider.* keys: manifest fields). So a bare `agent =` key is ALWAYS the abandoned terminology → remapping
is always correct and never destructive.

### Combined helper
```go
// remapAgentTerminology defense-in-depth-remaps the abandoned intermediate agent/[agent.*] terminology
// to provider/[provider.*] in raw TOML text BEFORE the typed decode (FR-B7). Pure + idempotent.
func remapAgentTerminology(data []byte) []byte {
    s := string(data)
    s = strings.ReplaceAll(s, "[agent.", "[provider.")   // (a) table headers
    s = agentKeyRe.ReplaceAllString(s, "${1}provider${2}") // (b) bare key, line-oriented
    return []byte(s)
}
```

---

## 4. Complement, not conflict — `migrateV2ToV3` (load.go:157)

`Load` calls `migrateV2ToV3(&cfg)` at load.go:157, AFTER `loadTOML` has decoded. The two are
COMPLEMENTARY and run in the correct order:
1. `loadTOML` (decode) → `remapAgentTerminology` runs on raw bytes BEFORE decode, so agent-keyed data
   SURVIVES into the typed Config (this fix). Without it, the data is gone before migrateV2ToV3 ever runs.
2. `migrateV2ToV3` (post-decode) → folds `default_provider` into the model slash-prefix (UNCHANGED).
   It remains a no-op for agent TERMINOLOGY (the remap already happened upstream in loadTOML).

So **migrateV2ToV3's LOGIC does not change** — only its DOC COMMENT (it currently claims agent remap is a
"NO-OP in memory" / "no agent data reaches cfg", which becomes FALSE after this fix: agent data IS
remapped to provider upstream). Update the doc comment to say so.

### migrate.go text to update (TWO spots — unique strings, edit-safe)
Spot A — the AGENT TERMINOLOGY doc-comment paragraph (migrate.go, in the `migrateV2ToV3` doc comment):
```
// AGENT TERMINOLOGY (FR-B7 "first map agent/[agent.*] → provider/[provider.*]"): a NO-OP in memory.
// fileConfig (file.go) decodes only the [provider] table (no Agent field) and toml.Unmarshal silently DROPS
// unknown [agent.*] tables — so no agent-keyed data ever reaches the typed Config. The real textual rewrite
// is the on-disk `config upgrade` command's job (P1.M3.T1.S2), which reads raw TOML where [agent.*] survive.
```
Spot B — the `(0)` implementation comment (migrate.go, top of migrateV2ToV3's body):
```
	// (0) agent→provider: documented no-op (no agent data reaches cfg — see doc comment).
	// fileConfig has NO Agent field; loadTOML uses toml.Unmarshal which silently drops unknown
	// [agent.*] tables. The real [agent.*]→[provider.*] rewrite is S2's on-disk config upgrade.
```
Both currently assert the remap is a no-op; after this fix they must state the remap is handled
textually upstream in `loadTOML` (`remapAgentTerminology`), so agent-keyed data reaches cfg as provider,
and migrateV2ToV3 itself needs no agent logic (the upstream remap already ran).

---

## 5. Test plan — reuse `writeTempTOML` + the `TestLoadTOML*` pattern (file_test.go)

`file_test.go` is `package config` (internal) → can call `loadTOML` directly. It already has:
- `writeTempTOML(t, body) string` — writes body to `t.TempDir()/config.toml`, returns path. REUSE it.
- `TestLoadTOMLValid` / `TestLoadTOML_V2Fields` — the canonical loadTOML assertion pattern. MIRROR it.
- imports already present: `bytes, fmt, os, path/filepath, strings, testing, time`. **No new test-file
  imports needed** (`strings` + `testing` cover the new tests).

### Test 1 — integration (the contract's explicit ask): `TestLoadTOML_AgentTerminologyRemapped`
Body (the contract's v2 fixture):
```toml
config_version = 2

[defaults]
agent = "pi"

[agent.pi]
default_model = "glm-5.2"
```
Write via `writeTempTOML`, call `loadTOML`, assert:
- `cfg.Provider == "pi"` (today: `""` — the bug).
- `cfg.Providers["pi"]` non-empty AND `cfg.Providers["pi"]["default_model"] == "glm-5.2"` (today: missing).

### Test 2 — regression/idempotency (the contract's second ask): inside the same test OR a focused one
Assert a NORMAL `[provider.pi]` file is unaffected. The existing `TestLoadTOMLValid` ALREADY covers the
provider-terminology decode, so add a focused check that the remap is a no-op on provider text — easiest
as a direct helper unit test (Test 3).

### Test 3 — helper unit test (TDD precision): `TestRemapAgentTerminology` (table-driven)
Covers the helper directly (no file I/O) for idempotency + precision:
| case | input (snippet) | want (snippet) |
|------|-----------------|----------------|
| header | `[agent.pi]` | `[provider.pi]` |
| key (spaced) | `agent = "pi"` | `provider = "pi"` |
| key (tight) | `agent="pi"` | `provider="pi"` |
| indented key | `  agent = "pi"` | `  provider = "pi"` |
| comment untouched | `# agent = "pi"` | unchanged |
| value untouched | `model = "agent"` | unchanged |
| already-provider (idempotent) | `[provider.pi]\nprovider = "pi"` | unchanged |
| both header+key | `[agent.pi]\nagent = "pi"` | `[provider.pi]\nprovider = "pi"` |
| double-run idempotent | remap(remap(x)) == remap(x) | true |

---

## 6. Validation commands (verified against the module)

```bash
# Targeted (fast feedback):
go test ./internal/config/ -run 'TestLoadTOML_AgentTerminologyRemapped|TestRemapAgentTerminology' -v
# Full config suite (no regression — esp. TestLoadTOMLValid, TestLoadTOML_V2Fields, TestMigrateV2ToV3):
go test ./internal/config/ -v
# Whole tree (proves the migrateV2ToV3 doc-comment edit + new helper don't break anything):
go build ./... && go test ./...
# Style:
gofmt -l internal/config/        # must be empty
go vet ./internal/config/
# go.mod/go.sum UNCHANGED (regexp + strings are stdlib):
git diff --exit-code go.mod go.sum
# Scope check — exactly 3 files changed:
git status --porcelain
```

go.mod: module `github.com/dustin/stagecoach`. NO new deps (`regexp` + `strings` are stdlib).

---

## 7. Confidence & risks

**Confidence: 9.5/10** for one-pass success. Rationale:
- The decode seam (`loadTOML` → `toml.Unmarshal`) is clear and the insertion point is unambiguous.
- The textual remap is the ESTABLISHED house pattern (cmd/config.go `agentHeaderRe`/`rewriteV2ToV3` do the
  on-disk twin); the in-memory version reuses the same regexp idiom.
- The fix populates EXACTLY the fields the contract asserts (`cfg.Provider`, `cfg.Providers["pi"]`),
  traceable through `fileDefaults.Provider` + `materialize`'s whole-map `c.Providers = fc.Provider` copy.
- Test infra (`writeTempTOML`, `TestLoadTOML*` pattern) already exists; no new test-file imports.

**Risks (low):**
- **Import addition.** Must add BOTH `regexp` and `strings` to file.go (neither is imported today).
  Forgetting one → compile error. Mitigated: PRP lists both explicitly.
- **Doc-comment drift.** The two migrate.go comments still claim "no-op"/"no agent data reaches cfg".
  If left unchanged, the code contradicts its docs (a maintenance trap) — the contract mandates the
  update. Mitigated: PRP quotes both unique strings for exact edit.
- **Over-eager key remap.** A naive `strings.ReplaceAll("agent", "provider")` would corrupt values/
  comments. Mitigated: the PRP mandates the line-oriented `(?m)^(\s*)agent(\s*=)` regexp (key-name only).
- **Parallel coordination (P1.M4.T1.S1).** That task edits `bootstrap.go` + `bootstrap_test.go` ONLY;
  this task edits `file.go` + `migrate.go` + `file_test.go`. **Zero file overlap.** Safe to run in parallel.
