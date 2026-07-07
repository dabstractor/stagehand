# yaml.v3 Node API — comment-preserving `customCommands` upsert (VERIFIED empirically)

**Library:** `gopkg.in/yaml.v3` **v3.0.1** (archived upstream 2025 — no fixes coming; design around it).
**Verification method:** throwaway Go program in `/tmp/yamlverify` (go 1.22, yaml.v3 v3.0.1) run against
hand-maintained lazygit-style configs. **Verified 2026-07-02** (FR-D5 discipline). This file gates
P1.M4.T2.S2 (FR-I3 comment-preserving edit + FR-I5 lazygit entry). Cross-references
`architecture/external_deps.md §2` (the same claims, now reproduced with executable proof).

The import path is exactly `gopkg.in/yaml.v3`. NOTE: `go.sum` already carries a *transitive*
`go.yaml.in/yaml/v3 v3.0.4` (a cobra/pflag pull); the DIRECT dependency to add is `gopkg.in/yaml.v3`.

---

## 1. Node-tree structure (confirmed)

`yaml.Unmarshal(docBytes, &root)` where `root yaml.Node`:

- `root.Kind == yaml.DocumentNode` (1), `len(root.Content) == 1`, `root.Content[0]` is the **top-level
  MappingNode** (Kind 4). (For a one-item doc like `- key: ...`, `root.Content[0]` is a SequenceNode.)
- A **MappingNode.Content is a flat slice read in PAIRS**: `Content[0]` = first key-scalar,
  `Content[1]` = its value-node, `Content[2]` = second key-scalar, `Content[3]` = its value-node, …
- A **SequenceNode** (`Tag == "!!seq"`) `.Content` is a flat slice of item nodes (one MappingNode per
  `- …` list item); its `.Tag` is `"!!seq"`, an item MappingNode's `.Tag` is `"!!map"`.

Verified tree for:
```yaml
gui: {…}
customCommands:
  - key: 'b' …
git: {…}
```
```
root.Kind=Document  Content=1
  topMap.Kind=Mapping  Content(pairs)=6        # 3 keys × (key,value)
    pair 0: key="gui"            valKind=Mapping
    pair 2: key="customCommands" valKind=Sequence
    pair 4: key="git"            valKind=Mapping
  seqNode.Kind=Sequence Tag="!!seq" Content(items)=1
      item 0 Kind=Mapping Content=6            # 3 fields × (key,value)
        first value: "b"  LineComment=""
```

## 2. LOCATE the `customCommands` sequence — walk pairs, NEVER hardcode the index

```go
func locateCustomCommands(topMap *yaml.Node) *yaml.Node {
    for i := 0; i+1 < len(topMap.Content); i += 2 { // PAIRS
        if topMap.Content[i].Kind == yaml.ScalarNode && topMap.Content[i].Value == "customCommands" {
            v := topMap.Content[i+1]
            if v.Kind == yaml.SequenceNode {
                return v
            }
        }
    }
    return nil // key absent, or not a sequence
}
```

**CRITICAL GOTCHA (verified by failure):** `topMap.Content[1]` is the VALUE of the FIRST key (`gui`),
NOT `customCommands`. A remove that hardcoded `root.Content[0].Content[1]` mutated the wrong node and left
the marked entry untouched. ALWAYS walk in pairs. (This bit me in the throwaway test — reproduce the
discipline in the implementation.)

## 3. Find the MARKED entry — `LineComment` on the first VALUE scalar

The stagecoach marker is a `LineComment` on the **value scalar of the `key` field** (item.Content[1]).
Stagecoach's identity is the marker, NOT the binding value.

```go
// item is one MappingNode in the sequence; Content[0]=key-name-scalar ("key"), Content[1]=value ("<c-a>")
func isStagecoachItem(item *yaml.Node) bool {
    return item.Kind == yaml.MappingNode && len(item.Content) >= 2 &&
        strings.Contains(item.Content[1].LineComment, "stagecoach-integration")
}
```
Verified: an entry built from `- key: '<c-a>' # stagecoach-integration` yields
`item.Content[1].LineComment == "# stagecoach-integration"` — yaml.v3 keeps the `# ` prefix in the stored
LineComment, so match on the **substring** `stagecoach-integration` (robust to the prefix).

## 4. BUILD the new entry node — unmarshal a one-item sequence doc, take `.Content[0].Content[0]`

```go
const entryTpl = `- key: '%s' # stagecoach-integration
  context: 'files'
  command: 'stagecoach'
  loadingText: 'Generating commit message…'
  output: 'none'
  description: 'stagecoach: AI commit'
`
func newEntryNode(key string) (*yaml.Node, error) {
    var doc yaml.Node
    if err := yaml.Unmarshal([]byte(fmt.Sprintf(entryTpl, key)), &doc); err != nil {
        return nil, err
    }
    // doc is DocumentNode; doc.Content[0] is the one-item SequenceNode; [0] is the MappingNode item.
    return doc.Content[0].Content[0], nil // Kind=Mapping Tag="!!map"
}
```
Verified: `newEntry.Kind=Mapping Tag="!!map" Content=12` (6 fields × 2), and
`newEntry.Content[1].LineComment == "# stagecoach-integration"`. Building from a YAML string (not hand-built
nodes) is simplest and produces the exact desired serialization.

## 5. INSERT vs REPLACE

```go
// REPLACE if a marked item exists, else APPEND (idempotent — replace, never duplicate).
replaced := false
for idx, item := range seq.Content {
    if isStagecoachItem(item) {
        seq.Content[idx] = newEntry // overwrite in place
        replaced = true
        break
    }
}
if !replaced {
    seq.Content = append(seq.Content, newEntry)
}
```

## 6. REMOVE the marked item (verified — surgical, other entries survive)

```go
for idx, item := range seq.Content {
    if isStagecoachItem(item) {
        seq.Content = append(seq.Content[:idx], seq.Content[idx+1:]...)
        break
    }
}
```
Verified against a 2-entry golden input: marked removed, the OTHER entry (`key: 'b'`) byte-for-byte
preserved, the unrelated `git.paging.colorArg` block intact, output reparses clean, marker substring
GONE. **If the sequence becomes empty after removal**, yaml.v3 serializes `customCommands: []` and it
round-trips fine — lazygit tolerates an empty `customCommands` list. Recommendation: **leave the empty
seq** (do NOT delete the `customCommands` key) — it minimizes the edit and `HasEntry()` is correctly
false. (Deleting the key entirely is also acceptable but a larger diff.)

## 7. ENCODE — `NewEncoder` + `SetIndent(2)` + `Encode(&documentNode)` + `Close()`

```go
var buf bytes.Buffer
enc := yaml.NewEncoder(&buf)
enc.SetIndent(2) // default is 4; lazygit configs use 2
if err := enc.Encode(&root); err != nil { return nil, err } // pass the DocumentNode (or root mapping)
if err := enc.Close(); err != nil { return nil, err }        // MUST close to flush
return buf.Bytes(), nil
```
Verified: comments survive (`# a user comment inside gui` round-tripped), quote styles survive
(`"echo existing"`, `'files'`), the new entry serializes with its marker inline
(`key: '<c-a>' # stagecoach-integration`). `enc.Close()` is REQUIRED (the encoder buffers).

**Gotcha (whole-doc normalization — architecture §2, reproduced):** yaml.v3 re-encodes the ENTIRE
document, so byte-identity outside the edited node is NOT guaranteed: a blank line between sections may
be dropped, and inline-comment spacing may be normalized. This is WHY the no-mangle *guarantee* lives in
`integrate.Apply` (FR-I3: preview+confirm shows any incidental normalization; re-parse validate +
auto-restore catches breakage) rather than in the serializer. **Implication for idempotency:** after
stagecoach's OWN write the file is in yaml.v3's canonical form, so a *second* Upsert returns
byte-identical bytes → `Apply` yields `OutcomeNoChange` (clean idempotency). A re-install against a
hand-maintained file yaml.v3 would normalize shows `OutcomeUpdated` with the normalization visible in the
diff — correct and confirmed-by-design.

## 8. CREATE-IF-MISSING — empty/absent file or absent `customCommands`

**Empty file:** `yaml.Unmarshal([]byte(""), &root)` → `root.Kind == 0` (null), `len(root.Content) == 0`.
Construct a fresh document:
```go
topMap := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{newEntry}}
topMap.Content = append(topMap.Content,
    &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "customCommands"}, seq)
doc := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{topMap}}
// encode doc — produces a clean config.yml with just the stagecoach entry.
```
**Missing `customCommands` key in an existing file:** locate fails → append the key-scalar + a new
SequenceNode (holding `newEntry`) to the EXISTING `topMap.Content` (do NOT rebuild the whole mapping —
that would discard the user's other top-level keys). Verified: append-to-existing-topMap preserves all
other keys.

`integrate.Apply` handles the missing-FILE case itself (FR-I3g: create-if-missing through the same
preview+confirm; `os.MkdirAll(filepath.Dir(path))` after confirm). The Target only needs to produce sane
bytes for the empty-input Parse/Upsert path.

## 9. CORRUPT YAML → refuse (verified)

`yaml.Unmarshal` returns a non-nil error for malformed YAML. Verified with
`gui:\n  show: [unclosed\n` → `*errors.errorString: yaml: line 1: did not find expected ',' or ']'`.
(For multi-error docs yaml.v3 returns `*yaml.TypeError`, which is also a non-nil error AND has a partial
node — but the error is non-nil, so **any non-nil error ⇒ refuse to write**.)

```go
func (t *lazygitTarget) Parse(data []byte) error {
    var root yaml.Node
    if err := yaml.Unmarshal(data, &root); err != nil {
        return err // Apply HARD-REFUSES (FR-I3a): "refused to write <path>: parse error …"
    }
    t.root = &root
    return nil
}
```
`Validate(data)` is the same probe on a throwaway instance (must NOT rely on prior Parse state, per the
`Target` contract): `var n yaml.Node; return yaml.Unmarshal(data, &n)`.

---

## Minimal end-to-end snippet (the whole adapter, condensed)

```go
import "gopkg.in/yaml.v3"

func topMap(root *yaml.Node) *yaml.Node { // root is DocumentNode; nil-safe for empty input
    if root == nil || root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
        return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"} // empty/missing → fresh map
    }
    return root.Content[0]
}

// Upsert returns new bytes with the stagecoach entry inserted/replaced.
func upsert(orig []byte, key string) ([]byte, error) {
    var root yaml.Node
    if err := yaml.Unmarshal(orig, &root); err != nil { return nil, err }
    m := topMap(&root)
    var seq *yaml.Node
    for i := 0; i+1 < len(m.Content); i += 2 { // PAIRS
        if m.Content[i].Value == "customCommands" && m.Content[i+1].Kind == yaml.SequenceNode {
            seq = m.Content[i+1]; break
        }
    }
    entry, err := newEntryNode(key); if err != nil { return nil, err }
    if seq == nil { // create key + seq on existing map (preserves other keys)
        seq = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
        m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag:"!!str", Value:"customCommands"}, seq)
    }
    replaced := false
    for i, it := range seq.Content {
        if isStagecoachItem(it) { seq.Content[i] = entry; replaced = true; break }
    }
    if !replaced { seq.Content = append(seq.Content, entry) }
    var buf bytes.Buffer; enc := yaml.NewEncoder(&buf); enc.SetIndent(2)
    if err := enc.Encode(&root); err != nil { return nil, err }
    return buf.Bytes(), enc.Close()
}
```
```
