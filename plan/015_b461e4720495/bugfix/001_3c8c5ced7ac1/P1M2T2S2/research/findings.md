# P1.M2.T2.S2 Research Findings — Backup file assertions in the config-upgrade tests

> Test-only task. Consumes the S1 fix (P1.M2.T2.S1 inserts `config.WriteTimestampedBackup(path)` into
> `runConfigUpgrade` before the `os.WriteFile`). This task adds the assertions that PROVE the backup is
> created on a real upgrade (and would fail without the S1 fix).

## §0 Contract recap

- **Input**: the fixed `runConfigUpgrade` (S1) which now creates a `config.toml.bak.<RFC3339-compact-UTC>`
  sibling holding the PRIOR content before overwriting, ONLY when `changed==true`.
- **Deliverable**: extend `TestConfigUpgrade_OlderUpdated` (config_test.go:1137) and
  `TestConfigUpgrade_V2ToV3Rewrite` (config_test.go:1423) with `filepath.Glob(config.toml.bak.*)` →
  ≥1 match assertions (clone config_test.go:631-634). Optionally assert backup content == pre-upgrade content.
- **Verify (do NOT break)**: `TestConfigUpgrade_AlreadyCurrent` (:1110) and `TestConfigUpgrade_InertFile_NoOp`
  (:1273) must still pass — they return early (`!changed` / `IsInert`) BEFORE the backup block, so they
  create NO backup.
- **Output**: updated `config_test.go`; passes with the S1 fix, FAILS without it (no backup → `len(matches)==0`).

## §1 The proven assertion pattern (clone this verbatim)

`internal/cmd/config_test.go:631-634` (and again at `:675-678`), inside the `config init --force` tests:
```go
// FR-B8: a timestamped backup of the prior config must exist alongside the written file.
matches, _ := filepath.Glob(filepath.Join(globalDir, "config.toml.bak.*"))
if len(matches) == 0 {
    t.Errorf("no timestamped backup created at %s/*.bak.* (FR-B8 reversible-write guarantee)", globalDir)
}
```
**`filepath` is ALREADY imported** (`config_test.go:8` `"path/filepath"`). `os` and `strings` are in scope.
**NO import change is needed.**

## §2 Path resolution (why `globalDir` is the right glob root)

- `setupNoRepo(t)` (config_test.go:26) sets `XDG_CONFIG_HOME=home` (a `t.TempDir()`) and returns
  `globalDir = filepath.Join(home, "stagecoach")`.
- `config.globalConfigPath()` (file.go) resolves to `$XDG_CONFIG_HOME/stagecoach/config.toml` =
  `globalDir/config.toml`.
- The upgrade writes to `config.ResolveConfigPath(flagConfig)` → same `globalConfigPath()` (no `--config`
  flag set in these tests). So the `.bak.<ts>` sibling lands in `globalDir/`. Globbing
  `filepath.Join(globalDir, "config.toml.bak.*")` is correct and matches the `config init --force` tests.

## §3 The exact insertion sites (verified line numbers + anchor text)

### Edit A — `TestConfigUpgrade_OlderUpdated` (current end of test, ~:1166-1172)
```go
	data, _ := os.ReadFile(config.GlobalConfigPath())
	content := string(data)
	if !strings.Contains(content, "config_version = 3") {
		t.Error("missing config_version = 3")
	}
	if !strings.Contains(content, "max_md_lines = 7") {
		t.Error("max_md_lines = 7 not preserved")
	}
}   // ← insert the backup assertion BEFORE this closing brace
```
- Pre-upgrade content (what the test wrote): `"config_version = 1\n[generation]\nmax_md_lines = 7\n"`.
- `writeConfigFile` (root_test.go:61) writes the body **verbatim** (`os.WriteFile`), so the backup holds
  exactly those bytes.

### Edit B — `TestConfigUpgrade_V2ToV3Rewrite`, first-run backup (between :1457 and the `// Re-run` comment)
```go
	if !strings.Contains(upgraded, "config_version = 3") {
		t.Errorf("on-disk config_version not 3:\n%s", upgraded)
	}
	   // ← insert the first-run backup assertion HERE
	// Re-run → no change (idempotent).
```
- Pre-upgrade content = the local var `v2` (a `config_version = 2` string the test builds).
- The backup is created on THIS first run (changed==true). Exact-equality `string(bak) == v2` is possible
  here because `v2` is in scope.

### Edit C — `TestConfigUpgrade_V2ToV3Rewrite`, no-spurious-2nd-backup (end of test, ~:1500-1503)
```go
	data2, _ := os.ReadFile(globalPath)
	if string(data2) != upgraded {
		t.Errorf("second run changed the file (not idempotent)")
	}
}   // ← insert the "exactly 1 backup after the idempotent re-run" assertion BEFORE this closing brace
```
- The 2nd run hits `!changed` → returns BEFORE the backup block → creates NO 2nd backup.
- After both runs, `len(matches)` MUST be exactly 1 (catches both "0" = S1 bug AND ">1" = a future
  regression that moves the backup above the `!changed` gate).

## §4 Why AlreadyCurrent + InertFile_NoOp need NO edit (just a green run)

- `TestConfigUpgrade_AlreadyCurrent` (:1110): writes `config_version = 3` → `upgradeConfigVersion` returns
  `changed==false` → `if !changed { return nil }` at the gate → the backup block is NEVER reached → no backup.
  The test asserts byte-identical content after the run (unchanged) — still true.
- `TestConfigUpgrade_InertFile_NoOp` (:1273): inert (all-commented) file → `config.IsInert` gate returns
  early → no backup. The test asserts no-op behavior — still true.
- Neither test currently asserts anything about backups. The S1 fix is behaviorally invisible to them
  (they `SetErr(io.Discard)` and don't glob). They pass UNCHANGED. **Optional strengthening**: add a
  `len(matches) == 0` negative assertion to each, to lock in the no-op-gate semantics. See PRP Task 5.

## §5 "Fails without the fix" — the core proof

Without the S1 fix, `runConfigUpgrade` never calls `WriteTimestampedBackup`, so no `.bak.*` file is created.
The new `len(matches) == 0 → t.Errorf(...)` in OlderUpdated + V2ToV3Rewrite therefore FAILS. With the S1
fix, exactly one backup is created on a real upgrade → the assertions PASS. This is the test's reason to
exist (it is the regression net for Issue 3 / FR-B8).

## §6 Scope fence

- Modifies ONLY `internal/cmd/config_test.go` (3 insertion sites; no new files; no new imports).
- Does NOT touch `internal/cmd/config.go` (S1 owns the production fix), `internal/config/backup.go`
  (the reused primitive), or any other test.
- No docs change (FR-B8 already documented; test-only).
