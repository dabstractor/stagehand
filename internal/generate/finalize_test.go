package generate

import (
	"context"
	"testing"

	"github.com/dabstractor/stagecoach/internal/config"
)

func TestApplyTemplate(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		tpl  string
		want string
	}{
		{"empty template is identity", "Fix parser", "", "Fix parser"},
		{"$msg-only template is identity", "Fix parser", "$msg", "Fix parser"},
		{"suffix template", "Fix parser", "$msg (#205)", "Fix parser (#205)"},
		{"prefix template", "Fix parser", "[skip ci] $msg", "[skip ci] Fix parser"},
		{"multiple $msg occurrences", "X", "$msg-$msg", "X-X"},
		{
			"multi-line message: full message substituted, suffix lands after body",
			"Sub\n\nBody line 1\nBody line 2",
			"$msg (#205)",
			"Sub\n\nBody line 1\nBody line 2 (#205)",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ApplyTemplate(tc.msg, tc.tpl)
			if got != tc.want {
				t.Errorf("ApplyTemplate(%q, %q) = %q, want %q", tc.msg, tc.tpl, got, tc.want)
			}
		})
	}
}

func TestFinalizeMessage(t *testing.T) {
	t.Run("empty cfg.Template is identity", func(t *testing.T) {
		cfg := config.Defaults()
		got := FinalizeMessage("Fix parser", cfg)
		if got != "Fix parser" {
			t.Errorf("FinalizeMessage = %q, want %q (byte-identical to today)", got, "Fix parser")
		}
	})

	t.Run("non-empty cfg.Template is applied", func(t *testing.T) {
		cfg := config.Defaults()
		cfg.Template = "$msg (#205)"
		got := FinalizeMessage("Fix parser", cfg)
		want := "Fix parser (#205)"
		if got != want {
			t.Errorf("FinalizeMessage = %q, want %q", got, want)
		}
	})
}

func TestStripCommentsAndTrim(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no comments", "line1\nline2", "line1\nline2"},
		{"strip comment lines", "keep\n# comment\nkeep2", "keep\nkeep2"},
		{"trim trailing whitespace", "keep   \t\nkeep2", "keep\nkeep2"},
		{"all comments returns empty", "# only comments\n# more", ""},
		{"mixed comments and whitespace", "# header\n\nkeep \t \n# footer\n", "keep"},
		{"empty input", "", ""},
		{"only whitespace lines", "   \n\t\n", ""},
		{"comment not inside kept line", "keep # inline\nkeep2", "keep # inline\nkeep2"},
		{"trailing newline", "keep\n", "keep"},
		{"carriage return trim", "keep\r\nkeep2\r", "keep\nkeep2"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := stripCommentsAndTrim(tc.input)
			if got != tc.want {
				t.Errorf("stripCommentsAndTrim(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestEditMessageNoop(t *testing.T) {
	t.Run("cfg.Edit==false is identity", func(t *testing.T) {
		cfg := config.Defaults() // Edit defaults to false
		msg := "Fix parser\n\nBody text"
		got, err := EditMessage(context.TODO(), msg, cfg, EditContext{})
		if err != nil {
			t.Fatalf("EditMessage(nop) unexpected error: %v", err)
		}
		if got != msg {
			t.Errorf("EditMessage(nop) = %q, want %q (byte-identity)", got, msg)
		}
	})
}
