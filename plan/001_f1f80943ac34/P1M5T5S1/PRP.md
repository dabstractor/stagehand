---
name: "P1.M5.T5.S1 — Review and update docs/ overview and any cross-cutting documentation"
description: |

  CREATE the repository `docs/` documentation set for Stagecoach v1.0 — the Mode-B "changeset-level
  documentation sync" task that runs LAST, with full visibility over every implementing subtask. Because
  `docs/` DOES NOT EXIST in the repo today (no "Mode A" docs subtasks created any docs/ files), this is
  NOT a review-and-tweak task: it authors the coherent, non-stale documentation set that the parallel
  README.md (P1.M5.T4.S1) links to as the "full reference (growing)". No Go source is written.

  CONTRACT (P1.M5.T5.S1, verbatim, key clauses):
    1. RESEARCH NOTE: "PRD §5 Mode B — cross-cutting docs that only make sense once the whole change is
       in place (architecture overviews, capability summaries). The PRD itself lives in docs/PRD.md and
       is READ-ONLY (never modify). Any NEW docs files created during implementation should be reviewed
       for consistency here."
    3. LOGIC: "Review all documentation files created or touched during v1.0: verify CLI flags match
       §15.2, exit codes match §15.4, config precedence matches §16.1, manifest fields match §12.1,
       install paths match §21.3. Ensure docs/PRD.md is untouched (read-only). Create or update docs/
       files as needed for a coherent documentation set. Verify the README feature blurbs and env-var
       sections are accurate. If no cross-cutting docs need changes beyond what Mode A subtasks already
       handled, document that decision."
    4. OUTPUT: "A coherent, non-stale documentation set for v1.0."
    5. DOCS: "[Mode B] This IS the changeset-level documentation sync task. It depends on every
       implementing subtask to ensure it runs last with full visibility."

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - `README.md` → P1.M5.T4.S1 (parallel; NOW EXISTS). VERIFY only — do not edit. docs/ must be
      CONSISTENT with README's claims (links, flags, precedence, install paths) but must not rewrite it.
    - `PRD.md` is at the REPO ROOT (NOT docs/PRD.md as the contract assumes) and is READ-ONLY. Link to
      it; NEVER move, copy, or edit it. Do NOT create a `docs/PRD.md` (would duplicate the canonical file).
    - All `*.go`, `Makefile`, `.goreleaser.yaml`, `.github/*`, `providers/*.toml`, `.gitignore`,
      `.markdownlint.json`, `tasks.json`, `prd_snapshot.md` → READ-ONLY / unchanged.
    - `install.sh`, `LICENSE` → DO NOT EXIST. Do not create (human/release-task owned).

  DELIVERABLE (CREATE one new directory + 5 Markdown files):
    CREATE docs/README.md            # overview + navigation index (the "docs/ overview")
    CREATE docs/cli.md               # full CLI reference (PRD §15)
    CREATE docs/configuration.md     # full configuration reference (PRD §16 + env + defaults + paths)
    CREATE docs/providers.md         # provider manifests reference (PRD §12)
    CREATE docs/how-it-works.md      # architecture overview (snapshot §13, safety/rescue §18, prompt §17)

  SUCCESS: `npx markdownlint-cli2 'docs/**/*.md'` → 0 errors against the EXISTING `.markdownlint.json`;
  every documented flag/exit-code/precedence/manifest-field/install-path is byte-accurate vs the shipped
  binary and Go source (cross-checked); README's claims are consistent with docs/ (no contradictions);
  `git status --short` shows ONLY the 5 new `docs/` files.

---

## Goal

**Feature Goal**: Ship a coherent, non-stale `docs/` documentation set for Stagecoach v1.0 — the
authoritative, browsable reference that fulfills the README's promise of a "full reference (growing)" and
that a new user (or integrator) can read top-to-bottom to understand every flag, exit code, config layer,
provider manifest field, and the snapshot-based architecture. Every fact in these docs is grounded in the
**already-shipped** binary and Go source (P1.M1–M4) and the PRD (READ-ONLY spec), because this Mode-B
task runs LAST with full visibility — it documents what exists, not what is planned.

**Deliverable**: FIVE new Markdown files under a new `docs/` directory:
- `docs/README.md` — overview + navigation index (the "docs/ overview" named in the title).
- `docs/cli.md` — CLI reference (PRD §15: synopsis, all 11 global flags, subcommands, exit codes, examples).
- `docs/configuration.md` — configuration reference (PRD §16: 7-layer precedence, file format, git-config
  keys, full env-var table, built-in defaults, config file paths).
- `docs/providers.md` — provider manifests reference (PRD §12: 18-field schema, command-rendering
  algorithm, the 6 verified built-ins, the tools-disable asymmetry, extensibility, output parsing).
- `docs/how-it-works.md` — architecture overview (PRD §13 snapshot flow + §13.4 diagram, §18 safety &
  rescue protocol, §17 prompt engineering) — the cross-cutting "architecture overview" Mode-B doc.

**Success Definition**:
- `npx markdownlint-cli2 'docs/**/*.md'` exits 0 (zero lint errors) against the repo's existing
  `.markdownlint.json` (`{default:true, MD013:false, MD033:false, MD060:false}`). *Tooling confirmed
  present: markdownlint-cli2 v0.22.1 via `npx`.*
- Every flag in `docs/cli.md` exists on `bin/stagecoach --help`; every exit code matches
  `internal/exitcode/exitcode.go`; the precedence list matches `config.go`'s
  `exampleConfigTemplate` header; every manifest field matches `internal/provider/manifest.go` toml tags;
  the 4 install paths match PRD §21.3 and `.goreleaser.yaml`. (Cross-check commands in Validation Loop L2.)
- README's claims (links to docs/, the precedence one-liner, the install commands, the provider list,
  the env-var mention) are CONSISTENT with docs/ — no contradiction. (README itself is NOT edited.)
- `docs/PRD.md` is NOT created (the canonical PRD lives at repo root and is read-only); docs/README.md
  links to the root `../PRD.md` instead.
- `git status --short` shows ONLY the 5 new `docs/*.md` files.

## User Persona

**Target User**: two audiences, both served by the same reference set:
1. The **plan-holder / end user** (PRD §7.1) who installed Stagecoach and wants the full flag/exit-code/
   config reference beyond `--help` — e.g. "what does exit 3 mean?", "how do I set a per-repo model?",
   "why is pi the default and not claude?".
2. The **multi-agent tinkerer / contributor** (PRD §7.3) adding a new agent via a `[provider.<name>]`
   manifest, who needs the authoritative 18-field schema + the 6 built-in manifests as templates.

**Use Case**: A user hits an unexpected exit code (3 = rescue) or wants to override the default model
per-repo, opens the matching `docs/` page, and finds the exact flag/git-config key/env var + a worked
example — without reading Go source or grepping the PRD.

**User Journey**: README "Full CLI and config reference" → `docs/` → `docs/cli.md` (flags/exit codes) or
`docs/configuration.md` (precedence/env) or `docs/providers.md` (manifests) or `docs/how-it-works.md`
(the snapshot architecture / safety guarantees) → back to the shell with the exact command.

**Pain Points Addressed**: `--help` is exhaustive but unstructured (no "why"); the PRD is the spec but
is not user-facing reference; a stale or absent docs/ leaves the README's "see docs/" link dangling.

## Why

- **Mode B — the documentation must reflect what shipped, last.** Per the contract, this task runs after
  every implementing subtask (M1–M4 + M5.T1–T4) so the docs are accurate, not aspirational. A doc that
  invents a flag the binary lacks (or omits one it has) is a defect; running last with cross-checks
  against the binary is how we prevent that.
- **docs/ does not exist; the README links to it.** The parallel README.md (P1.M5.T4.S1) ships a link,
  `See the [docs/](docs/) for the full reference (growing)`. An empty/absent docs/ is a dead promise;
  this task makes good on it with a focused, coherent set.
- **The cross-checks are the product.** The contract explicitly requires verifying CLI flags ↔ §15.2,
  exit codes ↔ §15.4, precedence ↔ §16.1, manifest fields ↔ §12.1, install paths ↔ §21.3. Each doc is
  built by READING the source and the PRD and reconciling them — the docs are the human-readable record
  of that reconciliation.
- **Architecture overview only makes sense at the end.** `docs/how-it-works.md` (snapshot flow, safety
  invariants, rescue protocol) is the cross-cutting "architecture overview" the Mode-B note calls out —
  it cannot be written until the plumbing, orchestrator, and rescue protocol all exist. They do now.

## What

Five Markdown files under a new `docs/` directory. Each is a focused, browsable reference grounded in
the shipped binary + Go source + PRD (read-only). The detailed contents, sources, and verbatim blocks
are in "Implementation Blueprint"; the file-by-file responsibilities are:

| File | Responsibility | Primary PRD source | Primary code source (verify against) |
|------|----------------|--------------------|----------------------------------------|
| `docs/README.md` | docs landing: 1-paragraph "what is Stagecoach", install (cross-ref README), file index, link to root PRD. | §5, §21.3 | (links only) |
| `docs/cli.md` | Full CLI reference: synopsis, all 11 global flags (table), subcommands, exit codes (table), examples, flag↔env↔git-config map. | §15.1–§15.5 | `internal/cmd/{root,providers,config,default_action}.go`, `internal/exitcode/exitcode.go` |
| `docs/configuration.md` | Full config reference: 7-layer precedence, config-file format, git-config keys, env vars (full table), built-in defaults, paths, provider overrides. | §16.1–§16.3 | `internal/config/config.go` (+ `exampleConfigTemplate` in `internal/cmd/config.go`) |
| `docs/providers.md` | Provider manifests: 18-field schema, command-rendering algorithm, 6 built-ins (table), tools-disable asymmetry, adding a new agent, output parsing. | §12.1–§12.9 | `internal/provider/{manifest,builtin,render,parse}.go`, `providers/*.toml` |
| `docs/how-it-works.md` | Architecture overview: snapshot flow + §13.4 diagram, atomic/safety invariants, rescue protocol + §18.3 message, prompt engineering summary. | §13, §17, §18 | `internal/generate/{generate,rescue}.go`, `internal/git/plumbing.go`, `internal/prompt/*.go` |

### Gap-handling decisions (REQUIRED — do not silently paper over)

These are facts about the repo today; the docs must handle each deliberately (full reasoning in
`research/docs_crosscheck_facts.md`):

- **GAP 1 — `docs/PRD.md` is NOT where the contract says it is.** The contract assumes the PRD lives at
  `docs/PRD.md`. REALITY: `PRD.md` is at the repo root and is READ-ONLY. **Do NOT move it; do NOT create
  `docs/PRD.md`.** `docs/README.md` links to the root `../PRD.md` and labels it "the authoritative product
  & technical specification (read-only)". This satisfies "ensure docs/PRD.md is untouched" by never
  creating one.
- **GAP 2 — there are NO "Mode A" docs to review.** No subtask created `docs/CONFIGURATION.md` /
  `docs/PROVIDERS.md`. The contract's "if no cross-cutting docs need changes beyond Mode A, document that
  decision" clause does NOT apply — this task authors the whole set. Document this explicitly in
  `docs/README.md`'s header note ("docs/ is new in v1.0").
- **GAP 3 — README has no dedicated env-var section.** The contract says "verify the README env-var
  sections are accurate." REALITY: README mentions env vars only inline, in the precedence one-liner
  (`CLI flags > STAGECOACH_* env vars > …`), which is ACCURATE but not exhaustive. No README edit is
  warranted (accurate; depth belongs in docs/). `docs/configuration.md` carries the FULL env-var table.
  Record this as a "verified, no change needed" finding in the PRP's success notes — do NOT edit README.
- **GAP 4 — namespace is `dustin/stagecoach`, not `dabstractor`.** `git remote` says `dabstractor`, but
  `go.mod`, `.goreleaser.yaml` (`owner: dustin`), and §21.3 all use `dustin/stagecoach`. Every install URL
  and cross-link in docs/ MUST use `dustin/stagecoach` (matching README + goreleaser). Matches README GAP A.
- **GAP 5 — `install.sh` does not exist yet.** The curl\|sh path is a release-time artifact. docs/ cross-
  references the README's install section (which already carries the "published with the first release"
  note) rather than re-asserting a working URL. Do NOT invent a different URL.
- **GAP 6 — `LICENSE` does not exist.** Do NOT assert a license in docs/. Omit license claims entirely.

### Success Criteria

- [ ] `docs/` directory exists with exactly the 5 files listed above.
- [ ] `npx markdownlint-cli2 'docs/**/*.md'` → 0 errors (against the existing `.markdownlint.json`).
- [ ] CLI flags cross-check (L2a): every flag in `docs/cli.md` is present in `bin/stagecoach --help`.
- [ ] Exit codes cross-check (L2b): the `docs/cli.md` exit-code table matches `internal/exitcode/exitcode.go` constants.
- [ ] Precedence cross-check (L2c): `docs/configuration.md` precedence matches `exampleConfigTemplate` header.
- [ ] Manifest fields cross-check (L2d): every field in `docs/providers.md` schema is in `manifest.go` toml tags.
- [ ] Install paths cross-check (L2e): the 4 install commands match §21.3 + `.goreleaser.yaml`.
- [ ] README consistency (L3): no contradiction between README's claims and docs/ (links resolve, flags/precedence/install match).
- [ ] `docs/PRD.md` is NOT created; `docs/README.md` links to the root `../PRD.md`.
- [ ] `git status --short` shows ONLY the 5 new `docs/*.md` files.

## All Needed Context

### Context Completeness Check

_Pass._ An author who has never seen this repo can implement this from: the 5-file spec (table above +
"Implementation Blueprint" per-file contents); the **verified facts table** in
`research/docs_crosscheck_facts.md` (every flag, exit code, env var, git-config key, default, path,
manifest field, built-in, install command — copy-pasteable and already cross-checked against source);
the verbatim PRD blocks (exit-code table, precedence ladder, manifest schema, §13.4 diagram, §18.3 rescue
block) in "Implementation Blueprint"; and a runnable validation loop (markdownlint + binary/source
cross-checks). No prior knowledge of the codebase required beyond reading the linked source files.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M5T5S1/research/docs_crosscheck_facts.md
  why: THE primary context doc. Every fact the docs must state — CLI flags, exit codes, config
       precedence, env vars, git-config keys, defaults, paths, all 18 manifest fields, the 6 built-ins,
       install paths, the namespace decision, the PRD-location discrepancy — already verified against
       source and copy-pasteable. Read this FIRST; it is the source for nearly every table in the docs.
  critical: §2 (PRD-location discrepancy → never create docs/PRD.md), §4 (markdownlint config + npx
            invocation), §5 (CLI surface), §6 (config model), §7 (manifest schema + 6 built-ins),
            §8 (install paths + namespace), §9 (how-it-works facts).

# --- the shipped code the docs must match (READ to quote accurately; cross-check against) ---
- file: internal/cmd/root.go
  why: the 11 §15.2 global flags (StringVar*/BoolVar* in init()) — the authoritative flag list for
       docs/cli.md. Note --version is cobra-auto from the Version field; --config is NOT a Config field.
  pattern: the flag registration block; each flag's help string is the user-facing description.
  gotcha: --version prints the ldflags-injected value ("dev" locally) — docs must not claim a release
          number; PersistentPreRunE is skipped for --help/--version.

- file: internal/cmd/providers.go
  why: `providers list` (NAME/DETECTED/DEFAULT table, ✓/✗, "(default)") + `providers show <name>`
       (merged TOML; exit 1 unknown). The Long: help strings are the user-facing descriptions.
  pattern: printProvidersList() format + resolvedDefault() (configured or first-detected built-in).

- file: internal/cmd/config.go
  why: `config init` (writes commented example, REFUSES overwrite → exit 1) + `config path` (prints
       global path). exampleConfigTemplate IS the canonical config reference — docs/configuration.md
       must agree with its precedence header + [defaults]/[generation]/[provider.X] sections.
  pattern: the precedence comment block (CLI > env > git-config > repo .stagecoach.toml > global > provider
           defaults > built-in defaults) + the env-var list + git-config keys.

- file: internal/cmd/default_action.go
  why: the success report format `[<7-sha>] <subject>` + `STATUS  path` file list; the FR18 auto-stage
       notice "Nothing staged — staging all changes (N files)."; the dry-run stdout=message/stderr=
       "(no commit created)"; the rescue/CAS/timeout/exit-code matrix in handleGenError().
  pattern: printCommitReport(), the auto-stage state machine, handleGenError() exit-code mapping.

- file: internal/exitcode/exitcode.go
  why: the AUTHORITATIVE §15.4 exit-code constants (Success=0, Error=1, NothingToCommit=2, Rescue=3,
       Timeout=124) + For()'s mapping order (timeout checked BEFORE rescue). docs/cli.md table source.
  gotcha: §15.4 OVERRIDES the architecture doc's generic table (2=nothing-to-commit, not usage; 3=rescue,
          not config). A timeout IS a *RescueError with Kind=ErrTimeout → maps to 124, not 3.

- file: internal/config/config.go
  why: Config struct (resolved value type) + Defaults() (the 7 Layer-1 values). docs/configuration.md's
       defaults table source. Note NoColor is toml:"-" (not a file field); Timeout is a time.Duration.
  pattern: the [defaults]/[generation]/[provider.<name>] grouping + snake_case toml leaf names.

- file: internal/provider/manifest.go
  why: the AUTHORITATIVE 18-field manifest schema (the toml tags). docs/providers.md schema table source.
  pattern: `toml:"…"` tags; fields defaulting when omitted (prompt_delivery→stdin, output→raw, …).

- file: internal/provider/builtin.go
  why: the 6 compiled-in manifests (pi/claude/gemini/opencode/codex/cursor) — the per-provider facts for
       docs/providers.md's built-in table (delivery, flags, default_model, bare_flags, tool-disable kind).

- file: internal/provider/registry.go   (preferredBuiltins)
  why: the auto-detect order ["pi","claude","gemini","opencode","codex","cursor"]; first detected = default.
       User-defined providers are NEVER auto-selected. docs/cli.md + docs/providers.md cite this.

- file: internal/provider/render.go + parse.go
  why: the command-rendering algorithm (§12.2) and the output-parsing pipeline (§12.9). docs/providers.md.

- file: providers/*.toml   (6 files)
  why: the shipped human-readable reference manifests. docs/providers.md points contributors at these as
       copy-paste templates; the docs' manifest example should mirror their field set + header style.

- file: .goreleaser.yaml
  why: confirms install-path namespaces (dustin/stagecoach, dustin/homebrew-tap, dustin/scoop-bucket) +
       the owner:dustin override of the git-remote. Anchors GAP 4.

- file: .markdownlint.json
  why: the EXISTING lint config (`{default:true, MD013:false, MD033:false, MD060:false}`) every doc must
       pass. Language hints on all fences; H1 first line; no duplicate headings.

- file: README.md   (P1.M5.T4.S1 — NOW EXISTS)
  why: the marketing surface docs/ must be CONSISTENT with (links, precedence one-liner, install
       commands, provider list, the docs/ link promise). READ-ONLY here — verify, do not edit.

# --- the PRD (authoritative spec — verbatim blocks sourced from here) ---
- doc: PRD.md §15.2   (the global-flags table)
- doc: PRD.md §15.3   (subcommands)
- doc: PRD.md §15.4   (the exit-code table — verbatim)
- doc: PRD.md §15.5   (example invocations — for docs/cli.md examples)
- doc: PRD.md §16.1   (the 7-layer precedence ladder — verbatim)
- doc: PRD.md §16.2   (the full config-file example)
- doc: PRD.md §16.3   (git-config keys)
- doc: PRD.md §12.1   (the manifest schema — verbatim field list)
- doc: PRD.md §12.2   (command-rendering algorithm)
- doc: PRD.md §12.3–§12.7 (the 6 built-in manifests)
- doc: PRD.md §12.7.1 (tools-disable asymmetry)
- doc: PRD.md §12.9   (output-parsing pipeline)
- doc: PRD.md §13.3   (snapshot invariants)
- doc: PRD.md §13.4   (the stage-while-generating diagram — verbatim)
- doc: PRD.md §18.1   (the safety invariant)
- doc: PRD.md §18.3   (the rescue message block — verbatim)
- doc: PRD.md §17.1/§17.4 (prompt engineering + raw-output rationale)
- doc: PRD.md §21.3   (the 4 install paths — verbatim)

# --- external (Markdown conventions + GitHub rendering) ---
- url: https://docs.github.com/en/get-started/writing-on-github/getting-started-with-writing-and-formatting-on-github/basic-writing-and-formatting-syntax
  why: GitHub-flavored Markdown (fenced code w/ language hints, tables, alerts `> [!NOTE]`).
  critical: fenced blocks need a language hint (```bash/```toml/```text) for MD040 + syntax coloring.
- url: https://github.com/DavidAnson/markdownlint/blob/main/doc/Rules.md
  why: the rule set behind markdownlint-cli2 v0.22.1 (the L1 gate). MD041 (H1 first line), MD024 (no
       duplicate headings), MD040 (fenced-language), MD009 (no trailing spaces) are ON; MD013/MD033/MD060
       are OFF per `.markdownlint.json`.
```

### Current Codebase tree (relevant slice)

```bash
README.md                      # EXISTS (P1.M5.T4.S1). docs/ must be consistent with it (READ-ONLY here).
PRD.md                         # the spec — at REPO ROOT (NOT docs/). READ-ONLY; link to it, don't move.
.markdownlint.json             # {default:true, MD013:false, MD033:false, MD060:false}. docs must pass it.
go.mod                         # module github.com/dustin/stagecoach; namespace = dustin.
.goreleaser.yaml               # install namespaces (dustin/*); owner:dustin overrides remote.
Makefile                       # `make build` -> ./bin/stagecoach (cross-check source).
internal/cmd/{root,providers,config,default_action}.go  # CLI surface (docs/cli.md).
internal/exitcode/exitcode.go                          # exit codes (docs/cli.md).
internal/config/config.go                              # Config + Defaults() (docs/configuration.md).
internal/provider/{manifest,builtin,registry,render,parse}.go  # manifests (docs/providers.md).
internal/generate/{generate,rescue}.go                 # orchestrator + rescue (docs/how-it-works.md).
internal/git/plumbing.go                               # write-tree/commit-tree/update-ref (how-it-works).
internal/prompt/*.go                                   # prompt assembly (how-it-works).
providers/{pi,claude,gemini,opencode,codex,cursor}.toml # shipped reference manifests.
docs/                          # DOES NOT EXIST — YOU CREATE THIS DIRECTORY + 5 files.
install.sh, LICENSE            # DO NOT EXIST — do not create (human/release owned).
```

### Desired Codebase tree with files to be added

```bash
docs/                          # CREATE directory.
├── README.md                  # CREATE — overview + navigation index.
├── cli.md                     # CREATE — full CLI reference (PRD §15).
├── configuration.md           # CREATE — full config reference (PRD §16 + env + defaults + paths).
├── providers.md               # CREATE — provider manifests reference (PRD §12).
└── how-it-works.md            # CREATE — architecture overview (snapshot §13, safety/rescue §18, prompt §17).
# (NO other files. NO docs/PRD.md, NO LICENSE, NO install.sh, NO edits to README/source/Makefile/release.)
```

### Known Gotchas of our codebase & Library Quirks

```markdown
# CRITICAL (#1) — NEVER create docs/PRD.md, NEVER move PRD.md. The contract says "docs/PRD.md" but the
#   PRD is at the REPO ROOT and is READ-ONLY. docs/README.md links to ../PRD.md. Creating docs/PRD.md
#   would duplicate the canonical file and violate "ensure docs/PRD.md is untouched".

# CRITICAL (#2) — DO NOT EDIT README.md. It is owned by P1.M5.T4.S1 (parallel, NOW EXISTS). This task
#   only VERIFIES README's claims are consistent with docs/ (GAP 3). If you find a contradiction, fix it
#   in docs/ (docs/ is the deeper reference), not by editing README.

# CRITICAL (#3) — MATCH THE BINARY, NOT YOUR MEMORY. Every flag/exit-code/precedence/field must be
#   cross-checked against the Go source + `bin/stagecoach --help` (Validation Loop L2). The §15.4 exit
#   codes OVERRIDE the architecture doc's generic table (2=nothing-to-commit, 3=rescue) — use exitcode.go.

# CRITICAL (#4) — NAMESPACE = dustin/stagecoach, NOT dabstractor. Every install URL and cross-link uses
#   dustin/stagecoach (matches go.mod + goreleaser owner:dustin + §21.3). A dabstractor URL is broken.

# GOTCHA (#5) — MARKDOWNLINT GATE. markdownlint-cli2 v0.22.1 runs via `npx` (NOT on bare $PATH — `which`
#   fails). Invoke: `npx markdownlint-cli2 'docs/**/*.md'`. The repo's .markdownlint.json disables MD013
#   (line length), MD033 (inline HTML), MD060 — but MD041 (H1 first line), MD024 (dup headings), MD040
#   (fenced-language), MD009 (trailing spaces) are ON. First line of every doc = `# Title`. Language hint
#   on every fenced block. No trailing whitespace. One H1 per file.

# GOTCHA (#6) — VERBATIM BLOCKS ARE CONTRACTUAL. The §13.4 diagram, the §15.4 exit-code table, the §16.1
#   precedence ladder, the §18.3 rescue block, and the §12.1 manifest field list must match the PRD /
#   source character-for-character (copy from "Implementation Blueprint"). Do NOT "tidy" the diagram's
#   box-drawing spacing; do NOT reorder the precedence ladder; do NOT invent manifest fields.

# GOTCHA (#7) — --version PRINTS "dev" LOCALLY. Do NOT document a release version. State plainly that
#   `stagecoach --version` prints the build version ("dev" for a local build; the release tag for a
#   released binary). Cross-ref README (which also avoids a hardcoded version).

# GOTCHA (#8) — DON'T PROMISE UNSHIPPED FEATURES. v1 is SINGLE-COMMIT; multi-commit decomposition is v2
#   (§10.3). install.sh + LICENSE do not exist yet. docs/ may mention these as "planned/at first release"
#   but must NOT document them as working. (install: GAP 5; license: GAP 6.)

# GOTCHA (#9) — SCOPE. This task creates exactly docs/{README,cli,configuration,providers,how-it-works}.md.
#   Do NOT edit README.md, PRD.md, *.go, Makefile, .goreleaser.yaml, .github/*, providers/*.toml,
#   .gitignore, .markdownlint.json, tasks.json, prd_snapshot.md. Do NOT create docs/PRD.md, LICENSE,
#   install.sh. If a gap tempts you to create/edit another file, instead document it (GAPs 1/3/5/6).

# GOTCHA (#10) — docs/ IS THE DEEPER REFERENCE, --help IS THE LIVING ONE. README says "--help and config
#   init are the authoritative, always-available reference; docs/ is the full reference (growing)." docs/
#   must be a SUPERSET in depth (the "why", tables, examples, architecture) and must never contradict
#   --help/config init. If `--help` and docs/ would disagree, FIX THE DOC (the binary is the source of
#   truth for shipped behavior), never the binary.
```

## Implementation Blueprint

### Data models and structure

_N/A — Markdown only. No data models/schemas/types. The "structure" is the 5-file docs/ tree (the table
in "What") and the section order within each file (specified per-file below).

### Verbatim PRD/source blocks (copy these EXACTLY into the docs)

These blocks are contractual (must match the PRD/source character-for-character). Copy from here.

**Block A — Exit codes (§15.4 ↔ exitcode.go)** — for `docs/cli.md`:

| Code  | Meaning                                                                                    |
| ----- | ------------------------------------------------------------------------------------------ |
| `0`   | Success (commit created, or dry-run message printed).                                      |
| `1`   | General error (generation failed, parse failed after retries, agent missing, CAS, usage).  |
| `2`   | Nothing to commit (clean tree after auto-stage, or nothing staged with `--no-auto-stage`). |
| `3`   | Rescue condition (snapshot taken, commit not created — manual recovery printed).           |
| `124` | Timeout (generation exceeded `--timeout`).                                                 |

_Note for the doc body_: "Exit codes mirror the constants in `internal/exitcode/exitcode.go`. A timeout
is reported as `124` (matching GNU `timeout`), not `3`."

**Block B — Config precedence (§16.1 ↔ exampleConfigTemplate header)** — for `docs/configuration.md`:

```text
CLI flags  >  STAGECOACH_* env vars  >  repo git config (stagecoach.*)  >
repo-local .stagecoach.toml  >  global config file  >  provider defaults  >  built-in defaults
```
(This is HIGH → LOW. Copy the §16.1 7-step ladder verbatim as an ordered list immediately after.)

**Block C — Manifest schema fields (§12.1 ↔ manifest.go toml tags)** — for `docs/providers.md`:
The 18 fields: `name`, `detect`, `command`, `subcommand`, `prompt_delivery`, `prompt_flag`,
`print_flag`, `model_flag`, `default_model`, `system_prompt_flag`, `provider_flag`, `default_provider`,
`bare_flags`, `output`, `json_field`, `strip_code_fence`, `retry_instruction`, `env` (a `[env]` subtable).
Present as a table (field | type | default | purpose), copied from PRD §12.1 + the `manifest.go` tags.

**Block D — Stage-while-generating diagram (§13.4)** — for `docs/how-it-works.md` (verbatim, ```text fence):

```text
Pane A (lazygit / shell)        Pane B (shell)
─────────────────────────       ───────────────────────
git add feature/login.js
stagecoach                     ┐
  ↳ snapshotting…             │  (user is free to work here)
  ↳ generating with pi…       │  git add docs/login.md
  ↳ (10s pass)                │  git add tests/login.test.js
  ↳ created abc1234           │  (these stay staged — NOT in abc1234)
                              ┘
                                stagecoach        # next run commits these
```

**Block E — Rescue message (§18.3)** — for `docs/how-it-works.md` (verbatim, ```text fence):

```text
❌ Commit generation failed.
------------------------------------------------------------
Your staged files were safely snapshotted before generation.
Tree ID: <TREE_SHA>

To commit the originally staged files manually:
  git commit-tree -p <PARENT_SHA> -m "Your message" <TREE_SHA> | xargs git update-ref HEAD

(omit "-p <PARENT_SHA>" if this is the repository's first commit)
------------------------------------------------------------
```

**Block F — Install paths (§21.3)** — for `docs/README.md` (verbatim; add the GAP 5 note under curl|sh):

```bash
# Homebrew (macOS / Linuxbrew)
brew install dustin/tap/stagecoach

# Go install (anywhere with Go)
go install github.com/dustin/stagecoach/cmd/stagecoach@latest

# Direct binary (curl|sh one-liner from GitHub Releases)
curl -fsSL https://github.com/dustin/stagecoach/raw/main/install.sh | bash

# Windows (Scoop)
scoop install dustin/stagecoach
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 0: VERIFY inputs (RUN, no edit) — author docs against SHIPPED behavior, not memory
  - RUN: `make build`                       # produces ./bin/stagecoach (the docs' commands must match THIS)
  - RUN: `./bin/stagecoach --help`           # capture the real flag list + descriptions (docs/cli.md)
  - RUN: `./bin/stagecoach --version`        # expect "dev" — do NOT document a release version
  - RUN: `./bin/stagecoach providers list`   # capture the NAME/DETECTED/DEFAULT table
  - RUN: `./bin/stagecoach providers show pi` # capture a real merged manifest (docs/providers.md example)
  - RUN: `./bin/stagecoach config path`      # capture the global config path string (docs/configuration.md)
  - RUN: `./bin/stagecoach config init` (in a throwaway dir; inspect exampleConfigTemplate) # config reference
  - READ: internal/exitcode/exitcode.go (exit codes); internal/config/config.go (Config+Defaults);
          internal/cmd/config.go (exampleConfigTemplate precedence header); internal/provider/manifest.go
          (18 toml fields); internal/provider/builtin.go (6 built-ins); .goreleaser.yaml (install ns);
          README.md (consistency); PRD §15/§16/§12/§13/§18/§21.3 (verbatim blocks).
  - NOTE every captured string so the docs quote the binary/source, not a guess.

Task 1: CREATE docs/ + docs/README.md (overview + navigation index)
  - CREATE the docs/ directory.
  - docs/README.md: H1 `# Stagecoach documentation`. One paragraph: what Stagecoach is (cross-ref README's
    §5 pitch — do NOT duplicate verbatim, summarize + link to ../README.md). An install block (Block F
    verbatim + GAP 5 note) OR a one-line "See the README for install" (preferred: link, since README is
    the install home — but include the 4 commands for self-containedness). A "Documentation index" table
    linking to cli.md / configuration.md / providers.md / how-it-works.md (one row each: link | one-line
    purpose). A note that docs/ is new in v1.0 (GAP 2). A link to ../PRD.md labelled "the authoritative
    product & technical specification (read-only)" (GAP 1 — NEVER create docs/PRD.md). A "Reporting
    issues / contributing" pointer to ../README.md.
  - GOTCHA: every internal link is relative (../README.md, cli.md, ../PRD.md, ../providers/). First line
    is the H1. No license claim (GAP 6).

Task 2: CREATE docs/cli.md (full CLI reference — PRD §15)
  - H1 `# CLI reference`. Sections (in order):
    1. Synopsis (§15.1): `stagecoach [flags]` / `stagecoach <command> [flags]`; default action paragraph
       (no command → commit staged; auto-stage-all if nothing staged & on). Link to how-it-works.md.
    2. Global flags (§15.2): a table — Flag | Env | Git config | Default | Description — copied from the
       §15.2 table AND cross-checked against root.go (all 11 flags incl. --version/--help). See the
       verified table in research/docs_crosscheck_facts.md §5.
    3. Subcommands (§15.3): `providers list`, `providers show <name>`, `config init`, `config path` —
       each with usage + output example (quote the real providers list table + config path string from
       Task 0). Note config init REFUSES overwrite (exit 1).
    4. Exit codes (§15.4): Block A table verbatim + the timeout-is-124 note.
    5. Flag ↔ env ↔ git-config map: a compact table (each config-backed flag: its env var + git-config
       key + which layers it lives in). Source: exampleConfigTemplate header.
    6. Examples (§15.5): the happy path, --provider/--model, git config persist, --dry-run, -a, the
       lazygit binding (cross-ref README), `--dry-run --no-color | tee`.
  - GOTCHA: --config and the behavioral flags (--all/-a, --no-auto-stage, --dry-run) have NO env/git-config
    analog — note that in the map. --version prints "dev" locally (GOTCHA #7).

Task 3: CREATE docs/configuration.md (full config reference — PRD §16)
  - H1 `# Configuration`. Sections (in order):
    1. Precedence (§16.1): Block B verbatim + the 7-step ordered ladder (built-in defaults → … → CLI flags).
    2. The config file: GLOBAL path ($XDG_CONFIG_HOME/stagecoach/config.toml, default
       ~/.config/stagecoach/config.toml) + REPO-LOCAL (./.stagecoach.toml, gitignored). `config init`
       writes the commented template; `config path` prints the global path. Note field-merge semantics
       for provider overrides (§16.1 last paragraph).
    3. File format (§16.2): the full [defaults]/[generation]/[provider.X] example (copy §16.2; it matches
       exampleConfigTemplate). Annotate the [generation] tuning fields with their defaults.
    4. Built-in defaults: a table (option | default | source) from config.go Defaults() — timeout 120s,
       auto_stage_all true, max_diff_bytes 300000, max_md_lines 100, max_duplicate_retries 3,
       subject_target_chars 50, output "raw", strip_code_fence true.
    5. Environment variables (FULL table — the GAP 3 depth): STAGECOACH_PROVIDER, STAGECOACH_MODEL,
       STAGECOACH_TIMEOUT, STAGECOACH_CONFIG, STAGECOACH_VERBOSE, STAGECOACH_NO_COLOR, NO_COLOR — each with
       meaning + example + which flag it mirrors. State these override the file, are overridden by flags.
    6. Git-config keys (§16.3): stagecoach.provider/model/timeout/auto_stage_all; example `[stagecoach]`
       block; `git config --get`/`--bool` notes; --local vs --global.
  - GOTCHA: the FILE uses string durations + [defaults]/[generation] subtables; Config is the RESOLVED
    form (Timeout is time.Duration). Document the file shape, not the struct. NoColor is toml:"-" (not a
    file field). --config is a discovery override, not a Config field.

Task 4: CREATE docs/providers.md (provider manifests — PRD §12)
  - H1 `# Provider manifests`. Sections (in order):
    1. What a manifest is (§12 intro): the agent-agnosticism layer; built-ins compiled in (zero config),
       user overrides field-merge. Link to providers/*.toml as human-readable references.
    2. The schema (§12.1): Block C — the 18-field table (field | type | default | purpose) copied from
       §12.1 + manifest.go tags. Note which fields are optional w/ defaults (prompt_delivery→stdin,
       output→raw, strip_code_fence→true, retry_instruction→"Output ONLY…").
    3. Command rendering (§12.2): the algorithm pseudocode (args assembly + stdin source) verbatim.
    4. The 6 built-ins (§12.3–§12.7): a table — provider | delivery | print_flag | model_flag |
       default_model | system_prompt_flag | tool-disable kind — one row each (pi/claude/gemini/opencode/
       codex/cursor). For each, the rendered command example. Cite auto-detect order
       (pi,claude,gemini,opencode,codex,cursor; first detected = default).
    5. The tools-disable asymmetry (§12.7.1): explicit-switch (pi, claude) vs read-only-constraint
       (codex, cursor, gemini); the safety consequence (§18.1 holds for all six). Quote §12.7.1's points.
    6. Adding a new agent (§12.8): the [provider.myagent] example; `providers show myagent` verification;
       `--provider myagent` use. Point at providers/pi.toml as the cleanest template.
    7. Output parsing (§12.9): the parseOutput pipeline (trim → strip fence → raw/json → JSON-in-prose
       fallback → newline normalize). Note raw is the v1 default contract.
  - GOTCHA: do NOT invent manifest fields — only the 18 in manifest.go. default_model is "" for
    opencode/codex/cursor (user must set). The TO CONFIRM notes from §12.7 belong in a "caveats"
    subsection, not as hardcoded facts.

Task 5: CREATE docs/how-it-works.md (architecture overview — §13 + §18 + §17)
  - H1 `# How Stagecoach works`. Sections (in order):
    1. The snapshot-based flow (§13.1–§13.3): why `git commit` is the wrong primitive (§13.1); the
       plumbing alternative — write-tree → commit-tree → update-ref CAS (§13.2); the 4 invariants from
       §13.3 (frozen content; later-staged files stay staged; atomic+safe; overlap-able latency).
    2. Stage-while-generating (§13.4): Block D diagram verbatim + the one-paragraph payoff.
    3. Safety & the rescue protocol (§18.1/§18.2/§18.3): the §18.1 invariant (no provider mutates the
       repo); the §18.2 failure-mode→exit-code table (cross-ref docs/cli.md); the §18.3 rescue block
       (Block E verbatim) + the candidate-message note.
    4. Prompt engineering (§17): the mature-repo system prompt (style learn from last 20, anti-reuse,
       ~50-char subject, multi-line rule); the new-repo conventional-commit fallback (§17.2); the user
       payload (§17.3); why raw output, not JSON (§17.4).
  - GOTCHA: this is the cross-cutting "architecture overview" Mode-B doc — it ties together git plumbing
    + orchestrator + rescue + prompt. Do NOT duplicate the full prompt templates verbatim (they live in
    PRD Appendix A) — summarize + link to ../PRD.md Appendix A.

Task 6: REVIEW for accuracy + consistency (READ the rendered docs; fix fiction, no scope creep)
  - For EVERY `stagecoach …`, `git …`, `brew …`, `go install …`, `scoop …` line: confirm it is real
    (matches Task 0 captures / §21.3 / §15.5 / .goreleaser.yaml). Fix any fiction.
  - Confirm the 6 verbatim blocks (A–F) are byte-identical to the PRP's "Verbatim PRD/source blocks".
  - Confirm GAP 1 (no docs/PRD.md; link to ../PRD.md), GAP 3 (README env-var mention accurate; full table
    in configuration.md), GAP 4 (dustin/stagecoach everywhere), GAP 5 (curl note), GAP 6 (no license).
  - Confirm README consistency (L3): every claim README makes that docs/ also makes agrees (precedence
    one-liner, install commands, provider list, the docs/ link resolves to docs/README.md).
  - Confirm no unshipped feature is documented as working (GOTCHA #8: single-commit; no install.sh/LICENSE).

Task 7: VALIDATE (run Validation Loop L1–L3; fix until green)
  - RUN: `npx markdownlint-cli2 'docs/**/*.md'` → 0 errors (against the existing .markdownlint.json).
  - RUN: the L2 cross-checks (flags/exit-codes/precedence/manifest-fields/install-paths vs binary+source).
  - RUN: the L3 README-consistency + dead-link + scope checks.
  - RUN: `git status --short` → ONLY the 5 new docs/*.md files.
```

### Implementation Patterns & Key Details

```markdown
# PATTERN — cross-check, then write. For each table (flags, exit codes, precedence, manifest fields,
# defaults, install paths), OPEN the source file (root.go / exitcode.go / config.go / manifest.go /
# builtin.go / .goreleaser.yaml) and copy the authoritative values. The PRD is the spec; the source is
# the shipped truth. When they differ on a shipped-behavior detail, the SOURCE wins (and you note it).

# PATTERN — docs/ is a SUPERSET in depth, never a contradiction. README says --help/config init are the
# always-available reference; docs/ is the browsable full reference. If docs/ and --help would disagree,
# the binary is right — fix the doc.

# PATTERN — every internal link is relative and resolves. docs/README.md → cli.md, configuration.md,
# providers.md, how-it-works.md, ../README.md, ../PRD.md, ../providers/. Verify each resolves to a file
# that exists (L3 dead-link audit). Do NOT link to docs/PRD.md (doesn't exist; GAP 1).

# PATTERN — language hints on every fenced block (```bash/```toml/```text/```ini/```yaml) for MD040 +
# GitHub syntax coloring. First line of each doc is an H1; exactly one H1 per file (MD025).

# PATTERN — use GitHub alerts (`> [!NOTE]` / `> [!IMPORTANT]`) for the caveats (GAP 5 curl note, the
# tools-disable asymmetry callout, the "docs/ is new in v1.0" note). They render cleanly and aren't links.

# PATTERN — tables for reference data (flags, exit codes, env vars, defaults, manifest fields, built-ins).
# Prose for concepts (snapshot flow, safety). Copy the PRD's table structure where it exists.

# PATTERN — the manifest example mirrors a SHIPPED manifest (providers/pi.toml's field set). The 6
# built-ins table mirrors providers/*.toml + builtin.go. Do NOT invent fields beyond manifest.go's 18.
```

### Integration Points

```yaml
NEW FILES (the ONLY artifacts — 5 Markdown files + the docs/ directory):
  - CREATE: docs/README.md
  - CREATE: docs/cli.md
  - CREATE: docs/configuration.md
  - CREATE: docs/providers.md
  - CREATE: docs/how-it-works.md

NO CONFIG CHANGES:
  - .markdownlint.json ALREADY EXISTS (`{default:true, MD013:false, MD033:false, MD060:false}`) and
    governs these docs. Do NOT add a per-docs lint config (the root one applies via markdownlint-cli2
    globbing). If a doc fails a rule that is ON, fix the doc — do not widen the exclusion.

DEPENDENCIES (the inputs these docs document — all Complete, so facts are stable):
  - CLI behavior         → P1.M4.T1.S2 (Complete): flags, subcommands, default action, success report.
  - providers commands   → P1.M4.T1.S3 (Complete): providers list / providers show.
  - config commands      → P1.M4.T1.S4 (Complete): config init / config path.
  - exit codes           → P1.M4.T1.S1 + P1.M4.T3.S3 (Complete): exitcode.For() mapping.
  - config model         → P1.M1.T4 (Complete): Config + Defaults + 7-layer precedence.
  - provider manifests   → P1.M2.T1/T2/T3 (Complete): schema + 6 built-ins + registry.
  - rendering/parsing    → P1.M2.T4/T6 (Complete): render + parseOutput.
  - snapshot/rescue      → P1.M1.T2 + P1.M3.T3/T4 (Complete): plumbing + rescue + orchestrator.
  - reference manifests  → P1.M5.T2.S1 (Complete): the 6 providers/*.toml files.
  - install paths        → P1.M5.T3.S2 (Complete): .goreleaser.yaml namespaces.
  - README               → P1.M5.T4.S1 (parallel, NOW EXISTS): the marketing surface docs/ must match.

HANDOFFS (do NOT create/edit — owned elsewhere):
  - README.md            → P1.M5.T4.S1. VERIFY consistency only; do not edit (GAP 3).
  - PRD.md (root)        → human-owned, READ-ONLY. Link to ../PRD.md; never move/copy (GAP 1).
  - Makefile, .goreleaser.yaml, .github/* → P1.M5.T3.* docs cite outputs, don't edit.
  - providers/*.toml     → P1.M5.T2.S1. docs point at them as templates; don't edit.
  - install.sh, LICENSE  → DO NOT EXIST. Human/release-task decision. Document, don't create (GAP 5/6).
  - *.go, .gitignore, .markdownlint.json, tasks.json, prd_snapshot.md → READ-ONLY / unchanged.
```

## Validation Loop

### Level 1: Markdown Lint (Immediate Feedback)

```bash
# markdownlint-cli2 v0.22.1 is available via `npx` (NOT on bare $PATH). It reads the repo-root
# .markdownlint.json ({default:true, MD013:false, MD033:false, MD060:false}) automatically.
npx markdownlint-cli2 'docs/**/*.md'
echo "exit=$?"        # expect 0

# Common friction (rules that ARE on): MD041 (first line must be H1), MD025 (one H1 per file),
# MD024 (no duplicate headings), MD040 (fenced block needs a language hint), MD009 (trailing spaces),
# MD007/MD005 (list indentation). Fix the doc to satisfy them — do NOT widen the exclusion.
# (MD013 line-length, MD033 inline-HTML, MD060 are already OFF, so the wide ASCII diagram + any inline
# HTML comment pass.)

# Also lint the docs together with README to catch cross-file duplicate-heading/anchor issues:
npx markdownlint-cli2 'docs/**/*.md' README.md
# Expected: exit 0, zero errors.
```

### Level 2: Accuracy Cross-Checks (the docs must match the BINARY + SOURCE)

```bash
# Build the binary the docs' commands must match.
make build
BIN=./bin/stagecoach

# (a) FLAGS — every flag in docs/cli.md exists on the binary (11 flags incl. --version/--help/-h):
$BIN --help | tee /tmp/help.txt
for f in --provider --model --config --timeout --verbose -v --no-color --all -a --no-auto-stage --dry-run --version --help; do
  grep -qw -- "$f" /tmp/help.txt && echo "OK: $f" || echo "MISSING flag: $f"
done
# Expected: all OK. (Every flag in docs/cli.md's flag table must be in this set.)

# (b) EXIT CODES — the docs/cli.md exit-code table matches internal/exitcode/exitcode.go constants:
grep -E 'Success|Error|NothingToCommit|Rescue|Timeout' internal/exitcode/exitcode.go | grep -E '= *[0-9]+'
# Expected: Success=0, Error=1, NothingToCommit=2, Rescue=3, Timeout=124. docs/cli.md Block A must match.

# (c) PRECEDENCE — docs/configuration.md precedence matches exampleConfigTemplate header:
grep -A2 'Resolution precedence' internal/cmd/config.go
# Expected: CLI flags > STAGECOACH_* env > repo git config > repo .stagecoach.toml > global file >
#           provider defaults > built-in defaults. docs/configuration.md Block B must match.

# (d) MANIFEST FIELDS — every field in docs/providers.md schema is in manifest.go toml tags (18 fields):
grep -oE 'toml:"[^"]+"' internal/provider/manifest.go | sed 's/toml:"//;s/"//' | sort -u | tee /tmp/fields.txt
#   for each field in docs/providers.md's schema table: grep -qw "<field>" /tmp/fields.txt
# Expected: 18 fields (name,detect,command,subcommand,prompt_delivery,prompt_flag,print_flag,model_flag,
#           default_model,system_prompt_flag,provider_flag,default_provider,bare_flags,output,json_field,
#           strip_code_fence,retry_instruction,env). docs must use ONLY these.

# (e) INSTALL PATHS — the 4 install commands in docs match §21.3 + .goreleaser.yaml:
grep -nE 'brew install dustin/tap/stagecoach|go install github.com/dustin/stagecoach|install.sh|scoop install dustin/stagecoach' docs/README.md docs/cli.md 2>/dev/null
grep -nE 'dustin/homebrew-tap|dustin/scoop-bucket|owner: dustin' .goreleaser.yaml
# Expected: install block present in docs/README.md (Block F verbatim); namespaces = dustin/* everywhere.

# (f) CONFIG PATHS + DEFAULTS — docs/configuration.md matches the binary + config.go:
$BIN config path                      # the global path string the doc quotes
grep -A14 'func Defaults' internal/config/config.go   # timeout 120s, auto_stage_all true, max_diff_bytes
                                      # 300000, max_md_lines 100, max_duplicate_retries 3, subject_target_chars
                                      # 50, output "raw", strip_code_fence true. docs must match.

# (g) PROVIDERS — docs/providers.md built-in table matches the binary + registry auto-detect order:
$BIN providers list                   # the 6 names + DETECTED + (default)
grep -n 'preferredBuiltins' internal/provider/registry.go   # [pi,claude,gemini,opencode,codex,cursor]
# Expected: docs list the same 6 in that order; note first-detected = default.
```

### Level 3: Completeness & Consistency (README + dead links + scope)

```bash
# (a) The 5 docs exist and each opens with an H1:
for f in README cli configuration providers how-it-works; do
  test -f docs/$f.md && head -1 docs/$f.md | grep -q '^# ' && echo "OK: docs/$f.md" || echo "BAD: docs/$f.md"
done

# (b) docs/README.md navigation links resolve to real files:
for l in cli.md configuration.md providers.md how-it-works.md ../README.md ../PRD.md; do
  target="docs/$l"; [ -e "$target" ] && echo "OK link: $l" || echo "DEAD link: $l"
done

# (c) GAP 1 — docs/PRD.md was NOT created; docs/README.md links to the root PRD:
test ! -e docs/PRD.md && echo "OK: no docs/PRD.md" || echo "FAIL: docs/PRD.md exists"
grep -q '\.\./PRD\.md' docs/README.md && echo "OK: links to ../PRD.md" || echo "FAIL: no PRD link"

# (d) GAP 4 — namespace = dustin/stagecoach everywhere in docs (no dabstractor):
grep -rni 'dabstractor' docs/ && echo "FAIL: dabstractor found" || echo "OK: no dabstractor"
grep -rni 'dustin/stagecoach' docs/ | head -1 && echo "OK: dustin/stagecoach present"

# (e) README consistency — docs/ and README agree on the shared facts:
grep -q 'CLI flags  >  STAGECOACH' docs/configuration.md   # precedence one-liner present
grep -q 'brew install dustin/tap/stagecoach' docs/README.md 2>/dev/null || grep -q 'brew install dustin/tap/stagecoach' docs/cli.md
grep -q 'pi, claude, gemini, opencode, codex, cursor' docs/providers.md   # auto-detect order

# (f) Dead-link audit — every URL/link in docs resolves or is explicitly a placeholder:
grep -roE 'https?://[^ )"]+|]\([^)]+\)' docs/ | sort -u
#   eyeball: external = dustin/stagecoach or github.com or shields.io; relative links resolve (b).

# (g) Mode-B honesty sweep — no unshipped feature documented as working:
grep -rniE '\-\-split|multi-commit hunk|install\.sh exists' docs/
#   any hit must be qualified ("planned for v2", "published at first release"). v1 = single-commit.
grep -rniE 'license|MIT|Apache' docs/   # GAP 6: no license assertion unless a real LICENSE file exists.

# (h) Scope discipline — ONLY the 5 new docs/*.md changed:
git status --short
# Expected: "?? docs/" (5 untracked files). NOTHING else (no .go, Makefile, .goreleaser.yaml, README.md,
#           PRD.md, .gitignore, .markdownlint.json, providers/*.toml, LICENSE, install.sh edits).
```

### Level 4: Render & Cohesion (GitHub rendering + doc-set coherence)

```bash
# (a) Render check — fenced blocks, tables, and GitHub alerts render (no broken fences):
npx markdownlint-cli2 'docs/**/*.md'   # a clean run covers structural well-formedness.

# (b) Cohesion sweep — the 5 docs cross-link sensibly (cli.md ↔ exit codes in how-it-works.md;
#     configuration.md ↔ config commands in cli.md; providers.md ↔ how-it-works.md safety):
grep -rE '\[.*\]\(cli\.md\)|\(configuration\.md\)|\(providers\.md\)|\(how-it-works\.md\)' docs/ | wc -l
# Expected: > 0 (the docs reference each other). Eyeball that each cross-link targets the right section.

# (c) Verbatim-block fidelity — the 6 blocks (A–F) match the PRP's "Verbatim PRD/source blocks":
grep -q '124' docs/cli.md && grep -q 'Nothing to commit' docs/cli.md       # Block A exit codes
grep -q 'CLI flags  >  STAGECOACH' docs/configuration.md                     # Block B precedence
grep -qw 'prompt_delivery' docs/providers.md && grep -qw 'bare_flags' docs/providers.md  # Block C schema
grep -q 'Pane A (lazygit / shell)' docs/how-it-works.md                     # Block D diagram
grep -q 'Tree ID: <TREE_SHA>' docs/how-it-works.md                          # Block E rescue
grep -q 'brew install dustin/tap/stagecoach' docs/README.md 2>/dev/null || grep -q 'README' docs/README.md  # Block F install or link

# (d) Title-deliverable match — "docs/ overview" exists (docs/README.md) and cross-cutting docs exist:
test -f docs/README.md && test -f docs/how-it-works.md && echo "OK: overview + cross-cutting present"
```

## Final Validation Checklist

### Technical Validation

- [ ] `npx markdownlint-cli2 'docs/**/*.md'` exits 0 (L1).
- [ ] L2 cross-checks all pass: flags (a), exit codes (b), precedence (c), manifest fields (d), install
      paths (e), config paths+defaults (f), providers (g).
- [ ] L3 checks pass: 5 files w/ H1 (a); nav links resolve (b); GAP 1 no docs/PRD.md + ../PRD.md link (c);
      GAP 4 namespace (d); README consistency (e); no dead links (f); no unshipped feature as working +
      no license assertion (g/h).
- [ ] L4 checks pass: clean render (a); docs cross-link (b); 6 verbatim blocks present (c); overview +
      cross-cutting docs exist (d).

### Feature Validation

- [ ] `docs/cli.md` documents all 11 §15.2 flags + §15.3 subcommands + §15.4 exit codes + §15.5 examples.
- [ ] `docs/configuration.md` documents the 7-layer precedence + file format + git-config keys + FULL env
      table (GAP 3 depth) + built-in defaults + global/repo-local paths.
- [ ] `docs/providers.md` documents the 18-field schema + rendering + 6 built-ins + tools-disable
      asymmetry + adding a new agent + output parsing.
- [ ] `docs/how-it-works.md` documents the snapshot flow + §13.4 diagram + safety/rescue (§18.3 block) +
      prompt engineering summary.
- [ ] `docs/README.md` is the overview + navigation index linking to the 4 above + ../PRD.md + ../README.md.
- [ ] Every documented command/flag/exit-code/field/path is byte-accurate vs the shipped binary + source.

### Code Quality & Scope Validation

- [ ] `git status --short` shows ONLY the 5 new `docs/*.md` files.
- [ ] No edits to README.md, PRD.md, *.go, Makefile, .goreleaser.yaml, .github/*, providers/*.toml,
      .gitignore, .markdownlint.json, tasks.json, prd_snapshot.md.
- [ ] No docs/PRD.md, LICENSE, or install.sh created (GAPs 1/5/6).
- [ ] GitHub-flavored Markdown conventions (language hints on all fences; H1 first line; one H1/file).
- [ ] Mode-B honest: documents shipped behavior; flags anything unshipped as "planned" (not as working).

### Documentation & Deployment

- [ ] docs/ is self-contained: a user can find any flag/exit-code/config-layer/manifest-field from docs/
      alone (+ the binary's --help/config init as the always-available backup).
- [ ] All internal links are relative and resolve; external links use github.com/dustin/stagecoach.
- [ ] The 6 GAPs are each handled deliberately (not silently papered over).
- [ ] docs/README.md explicitly notes docs/ is new in v1.0 (GAP 2) and links to the read-only root PRD.

---

## Anti-Patterns to Avoid

- ❌ Don't create `docs/PRD.md` or move `PRD.md` — the PRD is at the repo root, READ-ONLY (GAP 1). Link to
      `../PRD.md`. Creating docs/PRD.md duplicates the canonical file and violates "ensure docs/PRD.md is
      untouched".
- ❌ Don't edit `README.md` — it is owned by P1.M5.T4.S1 (parallel). This task VERIFIES consistency only
      (GAP 3); if docs/ and README would disagree, fix the doc.
- ❌ Don't document from memory — cross-check every flag/exit-code/precedence/field/path against the Go
      source + `bin/stagecoach --help` (Validation Loop L2). The §15.4 exit codes OVERRIDE the architecture
      doc's generic table (2=nothing-to-commit, 3=rescue); use `exitcode.go`.
- ❌ Don't invent manifest fields — only the 18 in `manifest.go` exist. The 6 built-ins table mirrors
      `builtin.go` + `providers/*.toml`. The TO CONFIRM notes from §12.7 are caveats, not hard facts.
- ❌ Don't use `dabstractor/stagecoach` anywhere — the public namespace is `dustin/stagecoach` (go.mod +
      goreleaser owner:dustin + §21.3). A dabstractor URL is a broken install (GAP 4).
- ❌ Don't assert a release version or a license — `--version` prints "dev" locally (GOTCHA #7); there is
      no LICENSE file (GAP 6). State the version behavior; omit license claims.
- ❌ Don't document an unshipped feature as working — v1 is single-commit; install.sh + LICENSE don't
      exist yet (GOTCHA #8). Mention as "planned/at first release", not as functional.
- ❌ Don't paraphrase the verbatim blocks (exit-code table, precedence ladder, §13.4 diagram, §18.3 rescue
      block, §12.1 schema, §21.3 install commands) — copy them character-for-character from "Implementation
      Blueprint" / the PRD / the source.
- ❌ Don't let `docs/` contradict `--help`/`config init` — the binary is the source of truth for shipped
      behavior; if they disagree, fix the doc, never the binary.
- ❌ Don't widen `.markdownlint.json` to silence a lint failure — the repo config is fixed; fix the doc to
      satisfy the rules that are ON (MD041/MD025/MD024/MD040/MD009).
- ❌ Don't create LICENSE, install.sh, docs/PRD.md, or edit source/Makefile/release files — this subtask is
      the 5 `docs/*.md` files ONLY.
- ❌ Don't skip markdownlint because "it's just docs" — `npx markdownlint-cli2` is the L1 gate; a lint
      error is a validation failure.
