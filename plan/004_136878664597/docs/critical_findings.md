# Critical Findings — Plan 004

## Finding 1: The delta is a single map-entry removal

`internal/config/roles.go` contains:
```go
var defaultRoleReasoning = map[string]string{
    "planner": "high",
}
```
Removing the `"planner": "high"` entry (or the entire map + its usage in `ResolveRoleModel`) is the
ENTIRE behavioral change. Off is the natural `""` zero value — no other role needed an entry before,
and now planner joins them.

The `ResolveRoleModel` function at roles.go:57-63 currently has:
```go
if reasoning == "" {
    reasoning = cfg.Reasoning // global fallback
}
if reasoning == "" {
    reasoning = defaultRoleReasoning[role] // FR-R6 shipped fallback: planner→high; others→""
}
```
After this change, the second `if` block is removed entirely. The global fallback (`cfg.Reasoning`,
which is `""` when unset, or `"off"` when config init wrote it) is the only layer beneath the per-role
override.

## Finding 2: Tests are the main bulk of work (not code)

The behavioral change is 1 map entry. The TEST changes are ~15 assertion flips across 4 test files:

- `internal/config/roles_test.go` — ~6 assertions renamed/flipped
- `internal/decompose/roles_test.go` — ~3 assertions renamed/flipped  
- `internal/cmd/default_action_test.go` — 1 assertion inverted
- `internal/config/bootstrap_test.go` — 1 new assertion added

**The safest approach:** remove the map entry, run `go test ./...`, and fix every failing assertion
the compiler/tests report. The test failures are exhaustive — no hand-audit needed.

## Finding 3: verbose_test.go is a FORMAT test, not a default test

`internal/ui/verbose_test.go:20` asserts `DEBUG: planner  p in pi (reasoning: high)` — but the test
fixture at line 14 explicitly sets `Reasoning: "high"` on the `RoleLine` struct. This tests the
`reasoningSuffix` formatter, NOT the default. It must NOT be changed.

## Finding 4: docs/ exists but Mode B is still unnecessary

The delta_prd.md incorrectly claims "no docs/ directory exists." In reality:
```
docs/cli.md
docs/configuration.md
docs/how-it-works.md
docs/providers.md
README.md
```

However, the delta_prd.md's CONCLUSION (no separate Mode B sweep task needed) is correct because:
- `docs/cli.md` — Mode A, touched by T1 (flag help)
- `docs/configuration.md` — Mode A, touched by T2 (config example)
- `docs/how-it-works.md` — no reasoning-default references
- `docs/providers.md` — documents manifest `reasoning_levels` shape, not defaults
- `README.md` — shows `--reasoning high` as an invocation example, not a default claim

Each docs file is a per-file change that rides with its implementing subtask. There is no cross-cutting
feature-overview doc to sweep. A final "Sync changeset-level documentation" task is added per §5 rules
but will be a verification no-op.

## Finding 5: Working tree already contains the implementation

The uncommitted working tree diff (`git diff`) already contains ALL the changes described above:
- `roles.go`, `config.go`, `bootstrap.go`, `root.go`, `pkg/stagecoach/stagecoach.go` — source changes
- `roles_test.go`, `bootstrap_test.go`, `decompose/roles_test.go`, `default_action_test.go` — test changes  
- `docs/cli.md`, `docs/configuration.md` — doc changes

**All tests pass** with these changes. The task breakdown describes the work that needs to be done;
the implementer should verify the working tree matches the contracts and finalize/commit.

## Finding 6: No git-config v2 keys (out of scope for this plan)

`internal/config/git.go` `loadGitConfig` reads ZERO v2 keys (no `stagecoach.role.*`, `stagecoach.reasoning`,
`stagecoach.commits`, etc.). This is a PRE-EXISTING gap from plan/002 that is NOT part of this delta.
Documented for awareness; not in scope for plan/004.
