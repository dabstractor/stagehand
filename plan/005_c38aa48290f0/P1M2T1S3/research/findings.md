# S3 research findings — format-mode prompt scaffolds + locale

## Inputs already landed
- **S1 config**: `config.Config.Format string` (config.go:85, values `auto|conventional|gitmoji|plain`,
  default `"auto"`, validated in load.go:356 `validateFormat`) and `config.Config.Locale string`
  (config.go:89, free-form, NOT validated, default `""`). No typed enum — compare against literal strings.
  Consumed as `cfg.Format` / `cfg.Locale`; NOT role-scoped (don't route through `ResolveRoleModel`).
- **S2 gitmoji**: `prompt.RenderGitmojiTable() string` (gitmoji.go:123) — §17.8 "emoji + meaning" block,
  one line `<emoji> - <description>`, 75 rows, NO trailing newline. `prompt.GitmojiTable` also exported.

## Builders to change (internal/prompt)
- `BuildSystemPrompt(examples, hasMultiline, subjectTarget)` (system.go:165) — §17.1 mature.
- `BuildFallbackPrompt(subjectTarget)` (system.go:160) — §17.2 new-repo/≤1-commit.
- `BuildPlannerSystemPrompt(examples)` (planner.go:80) — §17.5; contract const `plannerSystemPrompt`
  (planner.go:26-40) stays verbatim (FR-F5).
- `BuildArbiterSystemPrompt()` (arbiter.go:74) — zero-arg, UNCHANGED (decision prompt, no message).

`maturePromptHeader` (system.go:19-27) ends with the "Match the tone and style…from this repository:"
line. Split into `promptPreamble` + `examplesIntro`, reassemble via compile-time constant concat →
auto path byte-identical. Only non-ASCII byte today: em-dash (U+2014) in `antiReuseProhibition`.

## Four call sites (all thread cfg.Format, cfg.Locale — no logic change)
1. `internal/generate/generate.go` `buildSystemPrompt` (314/321/327) — message role.
2. `internal/decompose/message.go` `messageSystemPrompt` (229/236/242) — decompose message; also feeds the
   arbiter N+1 message via `generateMessage` → so arbiter N+1 inherits the fix.
3. `pkg/stagecoach/stagecoach.go` (395/402/408) — **third verbatim copy** of the helper (easy to miss).
4. `internal/decompose/planner.go` `callPlanner` (84) — planner FR-M11 single-shortcut message is emitted
   BY the planner (validated at planner.go:150 `single==true ⇒ message non-empty`), so format/locale must
   reach the planner system prompt.

No new branching needed at the sites: both BuildSystemPrompt and BuildFallbackPrompt dispatch to the
scaffold when `format != "auto"`, so the existing repo-age branch is untouched.

## Test patterns (stdlib only, no testify)
- Table-driven: `[]struct{name; …}` + `for _, tc := range { t.Run(tc.name, …) }`; `t.Errorf("%q"…)`.
- Canonical-exact: `internal/prompt/system_test.go` `TestBuildSystemPrompt_CanonicalExact` compares full
  output to a hand-built `const want`. KEEP these (update args to `…, "auto", ""`) — the FR-F1 proof.
- Stub-agent system-prompt capture: stub manifest (`stubtest.Manifest`) uses `PromptDelivery:"stdin"` and
  NO `system_prompt_flag`, so `provider.Render` PREPENDS the system prompt into the stdin payload
  (render.go:157). Capture via `t.Setenv("STAGECOACH_STUB_STDINFILE", file)` (read by
  cmd/stubagent/main.go:36) then `os.ReadFile`. Model: `internal/generate/generate_test.go:555-592`.
- Build/lint: `make test` (`go test -race ./...`), `make lint` (golangci-lint: errcheck,gosimple,govet,
  ineffassign,staticcheck,unused; Go 1.22). Module `github.com/dustin/stagecoach`. Coverage gate ≥85% on
  internal/{git,provider,generate,config}.

## Spec ambiguity resolved (documented in PRP)
Does `plain` retain the multi-line rule? §17.8 intro: non-auto modes retain "output rules, essence, and
multi-line rule". FR-F4: plain = "output rules + essence + subject-length target only" (reads narrower).
FR-F2 (conventional): "FR12 detection still runs". **Decision: retain the multi-line rule + subject target
in ALL non-auto modes** (follows the §17.8 intro + FR-F2). Pinned by a canonical test; a narrower reading
is a one-line change (skip the rule when `format=="plain"`).

Also: §17.8's conventional/gitmoji text bundles a "Target ~50" clause. To avoid duplicating the retained,
config-driven `subjectTargetLine`, the scaffold bodies OMIT their own target sentence. Backticks/markdown
in the §17.8 sketch are dropped in the actual prompt text (match §17.2's plain `type(scope): description`).
