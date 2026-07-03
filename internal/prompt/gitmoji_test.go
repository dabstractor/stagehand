package prompt

import (
	"strings"
	"testing"
)

func TestGitmojiTable_NonEmpty(t *testing.T) {
	if len(GitmojiTable) == 0 {
		t.Error("GitmojiTable is empty")
	}
}

func TestGitmojiTable_Count(t *testing.T) {
	if len(GitmojiTable) != gitmojiVerifiedCount {
		t.Errorf("len(GitmojiTable)=%d != gitmojiVerifiedCount=%d (refresh the const + date per Appendix E #16)",
			len(GitmojiTable), gitmojiVerifiedCount)
	}
}

func TestGitmojiTable_UniqueEmojis(t *testing.T) {
	seen := make(map[string]struct{}, len(GitmojiTable))
	for _, g := range GitmojiTable {
		if _, dup := seen[g.Emoji]; dup {
			t.Errorf("duplicate emoji %q", g.Emoji)
		}
		seen[g.Emoji] = struct{}{}
	}
	if len(seen) != len(GitmojiTable) {
		t.Errorf("unique emojis %d != table len %d", len(seen), len(GitmojiTable))
	}
}

func TestGitmojiTable_UniqueNames(t *testing.T) {
	seen := make(map[string]struct{}, len(GitmojiTable))
	for _, g := range GitmojiTable {
		if _, dup := seen[g.Name]; dup {
			t.Errorf("duplicate name %q", g.Name)
		}
		seen[g.Name] = struct{}{}
	}
	if len(seen) != len(GitmojiTable) {
		t.Errorf("unique names %d != table len %d", len(seen), len(GitmojiTable))
	}
}

func TestGitmojiTable_EveryEntryComplete(t *testing.T) {
	for i, g := range GitmojiTable {
		t.Run(string(rune('A'+i%26))+string(rune('0'+i/26)), func(t *testing.T) {
			if g.Emoji == "" {
				t.Errorf("entry %d (%q): Emoji is empty", i, g.Name)
			}
			if g.Description == "" {
				t.Errorf("entry %d (%q): Description is empty", i, g.Name)
			}
			if g.Name == "" {
				t.Errorf("entry %d: Name is empty", i)
			}
		})
	}
}

func TestRenderGitmojiTable(t *testing.T) {
	out := RenderGitmojiTable()

	// Non-empty.
	if out == "" {
		t.Fatal("RenderGitmojiTable returned empty string")
	}

	// Line count: len(GitmojiTable) lines means len(GitmojiTable)-1 newline separators.
	if got := strings.Count(out, "\n"); got != len(GitmojiTable)-1 {
		t.Errorf("newline count = %d, want %d (len(GitmojiTable)-1)", got, len(GitmojiTable)-1)
	}

	// No trailing newline (package convention: caller owns inter-block placement).
	if strings.HasSuffix(out, "\n") {
		t.Error("RenderGitmojiTable must NOT end with a trailing newline")
	}

	// Contains a known emoji + its description.
	if !strings.Contains(out, "🎨") {
		t.Error("output missing known emoji 🎨")
	}
	if !strings.Contains(out, "Improve structure / format of the code.") {
		t.Error("output missing known description")
	}

	// Every table emoji appears in the rendered output.
	for _, g := range GitmojiTable {
		if !strings.Contains(out, g.Emoji) {
			t.Errorf("output missing emoji %q (%s)", g.Emoji, g.Name)
		}
	}
}
