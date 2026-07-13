package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dabstractor/stagecoach/internal/integrate"
	"gopkg.in/yaml.v3" // v3.0.1 — archived upstream (2025); pinned for the Node API (HeadComment/LineComment/
	// FootComment) used for comment-preserving customCommands upsert. Verified 2026-07-02 against lazygit v0.62.2.
)

const (
	lazygitTargetName = "lazygit"
	defaultLazygitKey = "<c-a>"                  // FR-I5: the default key binding
	lazygitMarker     = "stagecoach-integration" // the LineComment substring that identifies stagecoach's entry
)

// entryTpl is the ONE stagecoach customCommands entry, as a one-item YAML sequence document.
// The %s is the key binding. The `# stagecoach-integration` marker rides on the `key` VALUE scalar
// (LineComment) — stagecoach's idempotency identity (FR-I3b), independent of the binding.
// Field names verified against lazygit v0.62.2 (external_deps.md §1): `output` (not the older
// subprocess/showOutput), `loadingText` valid, context `files`.
var entryTpl = `- key: '%s' # stagecoach-integration
  context: 'files'
  command: 'stagecoach'
  loadingText: 'Generating commit message…'
  output: 'none'
  description: 'stagecoach: AI commit'
`

var flagLazygitKey string // --key (local on integrateInstallCmd AND integrateRemoveCmd; mirrors --alias-name)

func init() {
	// Register --key on BOTH leaves for UI symmetry + resetIntegrateFlags parity with --alias-name.
	// (Remove targets the MARKER entry regardless of --key — the marker is stagecoach's identity; --key is the
	// binding stagecoach writes. Documented in docs/cli.md.) Shared var; default "" → resolved to "<c-a>".
	integrateInstallCmd.Flags().StringVar(&flagLazygitKey, "key", "",
		"lazygit key binding to install (default: <c-a>)")
	integrateRemoveCmd.Flags().StringVar(&flagLazygitKey, "key", "",
		"lazygit key binding (default: <c-a>; remove targets the marked stagecoach entry)")
}

// ---------------------------------------------------------------------------
// lazygitTarget — S1's integrate.Target adapter over yaml.v3's Node API.
// Owns ONLY the surgical node edit; integrate.Apply owns the no-mangle envelope
// (parse-first, preview-diff, confirm, backup, atomic write, validate+restore).
// ---------------------------------------------------------------------------

// lazygitTarget implements integrate.Target for lazygit's config.yml (PRD §9.21 FR-I5).
// key is the binding stagecoach writes; root is the parsed document (populated by Parse).
// Stateful: Parse populates root; HasEntry/Upsert/Remove read/mutate it;
// Validate is a clean local probe (no Parse reliance).
type lazygitTarget struct {
	key  string     // the binding ("<c-a>"); never "" (factory resolves)
	root *yaml.Node // parsed DocumentNode (nil before Parse)
}

// Marker returns the idempotency-identity substring (FR-I3b).
func (t *lazygitTarget) Marker() string { return lazygitMarker }

// Parse loads config.yml bytes into the node tree. Any error (incl. *yaml.TypeError) ⇒ refuse-to-write.
func (t *lazygitTarget) Parse(data []byte) error {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return err // Apply HARD-REFUSES: "refused to write <path>: parse error: <err>"
	}
	t.root = &root
	return nil
}

// HasEntry reports whether the marker-identified entry is present in the parsed tree.
func (t *lazygitTarget) HasEntry() bool { return t.findMarkedItem() != nil }

// Upsert returns new bytes with the stagecoach entry inserted (absent) or replaced (present).
// Surgical: only the marker entry changes semantically (incidental whole-doc normalization is
// possible — architecture §2 — and surfaced by Apply's preview-diff).
func (t *lazygitTarget) Upsert() ([]byte, error) {
	if t.root == nil {
		// Empty/missing file path — build a fresh document holding just the entry.
		t.root = &yaml.Node{Kind: yaml.DocumentNode,
			Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	}
	top := t.topMap()
	entry, err := t.newEntryNode()
	if err != nil {
		return nil, err
	}
	seq := t.locateSeq(top)
	if seq == nil {
		// customCommands key absent on an existing top map — append key + a new seq (preserves other keys).
		seq = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		top.Content = append(top.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "customCommands"}, seq)
	}
	// REPLACE the marked item if present (idempotent — never duplicate), else APPEND.
	replaced := false
	for i, it := range seq.Content {
		if isStagecoachItem(it) {
			seq.Content[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		seq.Content = append(seq.Content, entry)
	}
	return t.encode(t.root)
}

// Remove returns new bytes with the marker entry deleted.
// If customCommands becomes empty, it is left as `customCommands: []` (lazygit tolerates it;
// minimal edit; HasEntry==false). No marked entry ⇒ bytes unchanged.
func (t *lazygitTarget) Remove() ([]byte, error) {
	if t.root == nil {
		return nil, nil // nothing parsed (shouldn't happen — Apply only calls Remove when HasEntry)
	}
	top := t.topMap()
	seq := t.locateSeq(top)
	if seq == nil {
		return t.encode(t.root) // no customCommands at all — unchanged
	}
	for i, it := range seq.Content {
		if isStagecoachItem(it) {
			seq.Content = append(seq.Content[:i], seq.Content[i+1:]...)
			break // at most one marked entry (Upsert guarantees it)
		}
	}
	return t.encode(t.root)
}

// Validate re-parses on a throwaway (clean, side-effect-free; no Parse reliance).
// Apply calls it on written bytes.
func (t *lazygitTarget) Validate(data []byte) error {
	var n yaml.Node
	return yaml.Unmarshal(data, &n)
}

// --- lazygitTarget helpers ---

// topMap returns the top-level mapping (root.Content[0] for a DocumentNode), or a fresh map for empty input.
func (t *lazygitTarget) topMap() *yaml.Node {
	if t.root != nil && t.root.Kind == yaml.DocumentNode && len(t.root.Content) > 0 {
		return t.root.Content[0]
	}
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	if t.root == nil {
		t.root = &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{m}}
	} else {
		// Fix root to be a valid DocumentNode (empty-string Unmarshal sets Kind=0/null).
		t.root.Kind = yaml.DocumentNode
		t.root.Content = []*yaml.Node{m}
	}
	return m
}

// locateSeq walks the top mapping's Content IN PAIRS to find the customCommands sequence
// (NEVER a hardcoded idx — research/yaml-node-api.md §2).
func (t *lazygitTarget) locateSeq(top *yaml.Node) *yaml.Node {
	for i := 0; i+1 < len(top.Content); i += 2 {
		if top.Content[i].Kind == yaml.ScalarNode && top.Content[i].Value == "customCommands" &&
			top.Content[i+1].Kind == yaml.SequenceNode {
			return top.Content[i+1]
		}
	}
	return nil
}

// findMarkedItem returns the marked list item (or nil).
// The marker is on item.Content[1] (the `key` value scalar).
func (t *lazygitTarget) findMarkedItem() *yaml.Node {
	if t.root == nil {
		return nil
	}
	top := t.topMap()
	seq := t.locateSeq(top)
	if seq == nil {
		return nil
	}
	for _, it := range seq.Content {
		if isStagecoachItem(it) {
			return it
		}
	}
	return nil
}

// findKeyItem returns an UNMARKED item whose key value == key (for Foreign status detection), or nil.
func (t *lazygitTarget) findKeyItem(key string) *yaml.Node {
	if t.root == nil {
		return nil
	}
	seq := t.locateSeq(t.topMap())
	if seq == nil {
		return nil
	}
	for _, it := range seq.Content {
		if it.Kind != yaml.MappingNode || len(it.Content) < 2 {
			continue
		}
		// Content[0]="key" name, Content[1]=value; only the first field is the binding
		if it.Content[0].Value == "key" && it.Content[1].Value == key && !isStagecoachItem(it) {
			return it
		}
	}
	return nil
}

// newEntryNode builds the stagecoach entry from entryTpl and returns its single MappingNode.
func (t *lazygitTarget) newEntryNode() (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(fmt.Sprintf(entryTpl, t.key)), &doc); err != nil {
		return nil, fmt.Errorf("build stagecoach entry: %w", err)
	}
	return doc.Content[0].Content[0], nil // DocumentNode → SequenceNode → the one MappingNode item
}

// encode re-encodes the document with SetIndent(2) (lazygit convention; default is 4).
// enc.Close() is REQUIRED (yaml.NewEncoder buffers).
func (t *lazygitTarget) encode(root *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// isStagecoachItem reports whether a sequence item is stagecoach's (marker on the `key` value scalar).
func isStagecoachItem(item *yaml.Node) bool {
	return item.Kind == yaml.MappingNode && len(item.Content) >= 2 &&
		strings.Contains(item.Content[1].LineComment, lazygitMarker)
}

// ---------------------------------------------------------------------------
// lazygitEntry — S2's integrate.Entry. Wires lazygitTarget through integrate.Apply.
// ---------------------------------------------------------------------------

// lazygitEntry implements integrate.Entry for the lazygit target (PRD §9.21 FR-I5/I6).
// configPath is the resolved config.yml (prod: via resolveLazygitConfigPath; tests: an explicit tmp path).
// key is the binding.
type lazygitEntry struct {
	configPath string // resolved; tests set this directly to avoid touching the real config
	key        string // resolved (never "" — newLazygitEntry resolves "" → "<c-a>")
}

// newLazygitEntry builds the entry for the current invocation (reads the resolved --key + config path).
func newLazygitEntry() *lazygitEntry {
	key := flagLazygitKey
	if key == "" {
		key = defaultLazygitKey
	}
	return &lazygitEntry{configPath: resolveLazygitConfigPath(), key: key}
}

func (e *lazygitEntry) Name() string { return lazygitTargetName }

// Detect — FR-I2: lazygit requires lazygit on $PATH.
func (e *lazygitEntry) Detect(_ context.Context) error {
	if _, err := exec.LookPath("lazygit"); err != nil {
		return fmt.Errorf("lazygit not found on PATH: %w", err)
	}
	return nil
}

// ConfigPath — the resolved config.yml (list CONFIG column; display-only, best-effort).
func (e *lazygitEntry) ConfigPath(_ context.Context) (string, error) {
	if e.configPath == "" {
		e.configPath = resolveLazygitConfigPath()
	}
	return e.configPath, nil
}

// Status — FR-I1: marker present → Installed; unmarked item with our key → Foreign; else NotInstalled.
func (e *lazygitEntry) Status(_ context.Context) (integrate.Status, error) {
	data, err := os.ReadFile(e.resolvedPath())
	if err != nil {
		return integrate.StatusNotInstalled, nil // missing file ⇒ not installed (not an error)
	}
	tgt := &lazygitTarget{key: e.key}
	if perr := tgt.Parse(data); perr != nil {
		return integrate.StatusNotInstalled, nil // unparseable ⇒ not installed (install will refuse)
	}
	if tgt.HasEntry() {
		return integrate.StatusInstalled, nil
	}
	if tgt.findKeyItem(e.key) != nil {
		return integrate.StatusForeign, nil // a conflicting (unmarked) entry binds our key
	}
	return integrate.StatusNotInstalled, nil
}

// Install — FR-I5: drive the no-mangle protocol (integrate.Apply) to upsert the marker entry.
func (e *lazygitEntry) Install(ctx context.Context, opts integrate.InstallOptions) (integrate.InstallResult, error) {
	// FR-I4 / §9.21 parity: best-effort foreign-key probe BEFORE Apply. lazygitTarget.Upsert keys on the
	// MARKER; an UNMARKED entry already bound to our key is invisible to it and would be DUPLICATED
	// (customCommands is a YAML sequence — two entries can legally share a key). Surface it as a WARNING so
	// the user can pick --key. Mirrors gitAliasEntry.Install's foreign-conflict surfacing. Best-effort:
	// any read/parse failure simply skips the probe and falls through to Apply (Apply owns those paths).
	out := opts.Out
	if out == nil {
		out = os.Stderr
	}
	if data, rerr := os.ReadFile(e.resolvedPath()); rerr == nil {
		probe := &lazygitTarget{key: e.key} // throwaway — separate state from Apply's tgt
		if perr := probe.Parse(data); perr == nil {
			if probe.findKeyItem(e.key) != nil {
				fmt.Fprintf(out, "WARNING: a %s binding already exists (not managed by stagecoach); installing will create a duplicate customCommands entry — use --key to choose a different binding.\n", e.key)
			}
		}
	}

	tgt := &lazygitTarget{key: e.key}
	res, err := integrate.Apply(ctx, integrate.ApplyOptions{
		Path:    e.resolvedPath(),
		Target:  tgt,
		Action:  integrate.ActionUpsert,
		Yes:     opts.Yes,
		Out:     opts.Out,
		Confirm: opts.Confirm,
	})
	if err != nil {
		return integrate.InstallResult{}, err
	}
	return integrate.InstallResult{Outcome: res.Outcome, Target: e.Name(), Path: res.Path, Backup: res.Backup}, nil
}

// Remove — FR-I6: drive Apply with ActionRemove (deletes only the marker entry; restores entry-absence).
func (e *lazygitEntry) Remove(ctx context.Context, opts integrate.RemoveOptions) (integrate.RemoveResult, error) {
	tgt := &lazygitTarget{key: e.key}
	res, err := integrate.Apply(ctx, integrate.ApplyOptions{
		Path:    e.resolvedPath(),
		Target:  tgt,
		Action:  integrate.ActionRemove,
		Yes:     opts.Yes,
		Out:     opts.Out,
		Confirm: opts.Confirm,
	})
	if err != nil {
		return integrate.RemoveResult{}, err
	}
	return integrate.RemoveResult{Outcome: res.Outcome, Target: e.Name(), Path: res.Path, Backup: res.Backup}, nil
}

// resolvedPath returns the config path (e.configPath, resolving once if empty).
func (e *lazygitEntry) resolvedPath() string {
	if e.configPath == "" {
		e.configPath = resolveLazygitConfigPath()
	}
	return e.configPath
}

// resolveLazygitConfigPath discovers lazygit's config dir via `lazygit --print-config-dir`
// (short -cd; NO --config-dir), else falls back to XDG_CONFIG_HOME or the platform default
// (<userConfigDir>/lazygit/config.yml). Best-effort; never fatal.
//
// NOTE: when `lazygit` is on PATH, `--print-config-dir` resolves the config dir itself and
// ignores HOME/XDG_CONFIG_HOME. For scripting isolation, set XDG_CONFIG_HOME OR use a lazygit
// shim on PATH (validate.sh uses the shim approach). Without lazygit on PATH, this function
// correctly honors XDG_CONFIG_HOME → os.UserConfigDir → HOME/.config.
// external_deps.md §1 (VERIFIED 2026-07-02 v0.62.2).
func resolveLazygitConfigPath() string {
	if out, err := exec.Command("lazygit", "--print-config-dir").Output(); err == nil {
		if dir := strings.TrimSpace(string(out)); dir != "" {
			return filepath.Join(dir, "config.yml")
		}
	}
	// Fallback: honor XDG_CONFIG_HOME explicitly (os.UserConfigDir checks it on Linux,
	// but being explicit here ensures consistent behavior across platforms).
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "lazygit", "config.yml")
	}
	if ucd, err := os.UserConfigDir(); err == nil {
		return filepath.Join(ucd, "lazygit", "config.yml")
	}
	// last resort: HOME/.config/lazygit/config.yml
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "lazygit", "config.yml")
	}
	return "config.yml" // unreachable in practice
}
