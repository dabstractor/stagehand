# Research: Bootstrap `reasoning = "off"` — Three-Site Verification (P1.M1.T2.S1)

> **Purpose:** Verify the true live state of the three contract edit sites for P1.M1.T2.S1
> (`internal/config/bootstrap.go`, `internal/config/bootstrap_test.go`, `docs/configuration.md`),
> checked against HEAD on 2026-07-02. **HEADLINE: all three sites are ALREADY in the contract's
> target state — committed in `9d33b9e` ("make reasoning off by default for all roles"), the same
> commit that landed the sibling S1/S2 reasoning-default flip (Change A). The working tree is CLEAN.
> The contract's described STARTING state (reasoning absent / commented) DOES NOT EXIST in the live
> code. This task is a VERIFY-AND-CONFIRM runbook, NOT an insert task.**

---

## 1. Environment & Baseline

| Item | Value / Evidence |
|---|---|
| Module | `github.com/dustin/stagecoach`, `go 1.22` (cobra v1.10.2, go-toml/v2 v2.4.2) |
| Go / git | go1.26.4 / git 2.54.0 |
| Implementing commit | `9d33b9e make reasoning off by default for all roles` (HEAD-region; touches all 3 files) |
| Working tree (the 3 files) | **CLEAN** — `git status --porcelain` empty for all three (committed, not a stash/WIP) |
| Baseline test | `go test -race ./internal/config/` → **ok (cached)**. `go test ./...` green per system_context §1. |

**Implication:** The three edits the contract describes were already authored and committed. Re-applying
them would create DUPLICATES that break the build/tests. The PRP must redirect the implementer from
"insert these three things" to "verify they are present (once) and green."

---

## 2. The Three-Site Verification — all ALREADY target-state

### 2.1 Site (a): `internal/config/bootstrap.go` — the `[defaults]` writer ✓ DONE

The contract says: after the `provider` line and before the commented `# model` line, insert:
```
b.WriteString("reasoning = \"off\"   # off|low|medium|high; off by default for every role (FR-R6) — opt in per role below\n")
```

**Live state** (`bootstrap.go:120-128`, inside `buildBootstrapConfig`):
```go
	b.WriteString("[defaults]\n")
	fmt.Fprintf(&b, "provider = %q", target)
	if !isInstalledName(target, installed) {
		b.WriteString("  # no built-in agent detected on $PATH; defaulted to \"pi\" — edit if you use a different agent")
	}
	b.WriteString("\n")
	b.WriteString("reasoning = \"off\"   # off|low|medium|high; off by default for every role (FR-R6) — opt in per role below\n")   // ← PRESENT (line 127)
	b.WriteString("# model          = \"\"\n# timeout        = \"120s\"\n# auto_stage_all = true\n# verbose        = false\n")
```

**Byte-for-byte identical to the contract's specified string**, and correctly positioned (after `provider`,
before the commented `# model` block). There is EXACTLY ONE such line (`grep` → 1 match at :127).

### 2.2 Site (b): `internal/config/bootstrap_test.go` — `TestBuildBootstrapConfig_Pi` ✓ DONE

The contract says: add, after the `provider = "pi"` check:
```go
if !strings.Contains(content, "reasoning = \"off\"") { t.Error("missing uncommented reasoning = \"off\" in [defaults]") }
```

**Live state** (`bootstrap_test.go`, inside `TestBuildBootstrapConfig_Pi`, immediately after the
`provider = "pi"` assertion):
```go
	// reasoning = "off" uncommented in [defaults] (FR-B1 — emitted so the field is discoverable
	// in the generated file rather than hidden; off is the shipped default for every role, FR-R6)
	if !strings.Contains(content, `reasoning = "off"`) {
		t.Error("missing uncommented reasoning = \"off\" in [defaults]")
	}
```

**Semantically identical** to the contract's assertion (uses a raw-string literal `` `reasoning = "off"` ``
which equals the contract's `"reasoning = \"off\""`, and the same error message). A documenting comment
rides above it. PRESENT exactly once.

### 2.3 Site (c): `docs/configuration.md` — the config example `[defaults]` ✓ DONE

The contract says: move `reasoning` from commented (`# reasoning = "off"`) to UNCOMMENTED with comment
text `# off|low|medium|high; off by default for every role (FR-R6)`.

**Live state** (`docs/configuration.md:77-83`, the "Populated config" TOML example):
```toml
[defaults]
provider = "claude"
reasoning = "off"   # off|low|medium|high; off by default for every role (FR-R6)   ← UNCOMMENTED (line 80)
# model          = ""
# timeout        = "120s"
# auto_stage_all = true
# verbose        = false
```

**Uncommented, with the exact contract comment text.** The :64 prose and the :152-156 env-var table also
reference reasoning consistently. DONE.

---

## 3. The #1 Failure Mode — a DUPLICATE `reasoning = "off"` line

Because the contract is phrased as "INSERT …", an implementer who does not first read the live file will
add a SECOND `reasoning = "off"` line to `bootstrap.go`'s `[defaults]` writer. The consequences:

- **`TestBuildBootstrapConfig_Pi` would still PASS** (`strings.Contains` is satisfied by either copy).
- **`TestBuildBootstrapConfig_ValidTOML` would FAIL** — go-toml/v2 rejects a duplicate key in the same
  table: `toml: key 'reasoning' already exists` (the generated config would have two `reasoning =` lines
  under `[defaults]`). This test iterates 6 `(target, installed)` cases and unmarshals each, so the
  duplicate is deterministically caught.
- A duplicate in `docs/configuration.md` would be a visibly malformed example (two `reasoning =` lines).

**Mitigation:** the PRP front-loads "the line is ALREADY present; verify EXACTLY ONE; do NOT insert."
The authoritative gate is `grep -c 'reasoning = "off"'` (bootstrap.go → 1; the docs example → 1) plus the
green `TestBuildBootstrapConfig_ValidTOML`.

---

## 4. Provenance — why the work is already present

`git log --oneline -- internal/config/bootstrap.go docs/configuration.md` shows the most recent commit
touching both is:

```
9d33b9e make reasoning off by default for all roles
```

This single commit implemented BOTH Change A (the `defaultRoleReasoning` removal / `off`-for-all-roles
behavioral flip that S1/S2 verify) AND Change B (the three sites this task owns). The plan/004
`system_context.md §3` was authored as a PLANNING sketch against an older snapshot (it lists these sites
as "to change"), but the actual implementation commit `9d33b9e` landed all of Change B alongside Change A.
Hence the contract's "starting state" (reasoning absent/commented) does not exist in HEAD.

---

## 5. The Deterministic Verification Gates (the deliverable)

These are the gates an implementer runs to CONFIRM completion (no edits should be necessary):

```bash
cd /home/dustin/projects/stagecoach

# (1) bootstrap.go: EXACTLY ONE uncommented reasoning line, with the contract's comment text
grep -c 'reasoning = \\"off\\"   # off|low|medium|high; off by default for every role (FR-R6)' internal/config/bootstrap.go
#   → 1   (a 2 here means a DUPLICATE was added — revert it)

# (2) bootstrap_test.go: the assertion present (exactly once)
grep -c 'missing uncommented reasoning' internal/config/bootstrap_test.go
#   → 1

# (3) docs/configuration.md: reasoning UNCOMMENTED in the [defaults] example
grep -n '^reasoning = "off"' docs/configuration.md
#   → 80:reasoning = "off"   # off|low|medium|high; off by default for every role (FR-R6)
#   (a leading '# ' would mean it is still commented — it is NOT)

# (4) The full suite is green (the ValidTOML test is the duplicate-detector)
go test -race ./internal/config/ -run TestBuildBootstrapConfig
go test -race ./...
go build ./... ; go vet ./... ; gofmt -l .
```

If all five gates pass (they do at HEAD), the task is COMPLETE and ZERO source edits are required.

---

## 6. Decisions Log

| # | Question | Decision | Rationale |
|---|---|---|---|
| D1 | Insert the reasoning line per the contract? | **NO — it is already present.** | bootstrap.go:127 has it byte-for-byte; inserting again creates a duplicate that breaks TestBuildBootstrapConfig_ValidTOML (§3). |
| D2 | Add the test assertion per the contract? | **NO — it is already present.** | bootstrap_test.go `TestBuildBootstrapConfig_Pi` already asserts `reasoning = "off"` uncommented. |
| D3 | Uncomment reasoning in docs/configuration.md? | **NO — it is already uncommented.** | docs/configuration.md:80 is already `reasoning = "off"   # off|low|medium|high; off by default for every role (FR-R6)`. |
| D4 | What is the deliverable then? | **A verification pass + green gates.** | The work landed in commit `9d33b9e`. The PRP's value is (a) confirming completion honestly and (b) preventing the duplicate-line regression. |
| D5 | If somehow a site is NOT target-state? | Apply ONLY that one missing edit (no duplicates). | The gates pinpoint the exact site; a single surgical edit is safe. A blanket "apply all three" is NOT safe (it assumes an absent starting state that does not exist). |
