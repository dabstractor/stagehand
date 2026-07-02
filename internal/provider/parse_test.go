package provider

import "testing"

// TestParseOutput exercises the §12.9 pipeline against the full contract
// matrix from the PRP. It is white-box (same package) because parseOutput is
// unexported. Each case asserts both the returned message (exact string
// equality) and the ok flag exactly. Inline Manifest literals are used for
// determinism; the manifest fixture is built fresh per case so StripCodeFence
// and Output are explicit.
func TestParseOutput(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		m      Manifest
		want   string
		wantOK bool
	}{
		{
			name:   "raw clean",
			raw:    "feat: add parser",
			m:      Manifest{Output: "raw", StripCodeFence: true},
			want:   "feat: add parser",
			wantOK: true,
		},
		{
			name:   "fenced-raw",
			raw:    "```\nfeat: add parser\n```",
			m:      Manifest{Output: "raw", StripCodeFence: true},
			want:   "feat: add parser",
			wantOK: true,
		},
		{
			name:   "fenced-raw-with-langtag",
			raw:    "```text\nfeat: x\n```",
			m:      Manifest{Output: "raw", StripCodeFence: true},
			want:   "feat: x",
			wantOK: true,
		},
		{
			name:   "tilde fence",
			raw:    "~~~\nfeat: x\n~~~",
			m:      Manifest{Output: "raw", StripCodeFence: true},
			want:   "feat: x",
			wantOK: true,
		},
		{
			name:   "fenced-json",
			raw:    "```json\n{\"result\":\"feat: x\"}\n```",
			m:      Manifest{Output: "json", JSONField: "result", StripCodeFence: true},
			want:   "feat: x",
			wantOK: true,
		},
		{
			name:   "json-embedded-in-prose",
			raw:    "Sure! {\"result\":\"feat: x\"} hope this helps",
			m:      Manifest{Output: "json", JSONField: "result", StripCodeFence: true},
			want:   "feat: x",
			wantOK: true,
		},
		{
			name:   "malformed-json fallback to raw",
			raw:    "{\"result\": feat: x}",
			m:      Manifest{Output: "json", JSONField: "result", StripCodeFence: true},
			want:   "{\"result\": feat: x}",
			wantOK: true,
		},
		{
			name:   "json-non-string-field fallback",
			raw:    "{\"result\": 42}",
			m:      Manifest{Output: "json", JSONField: "result", StripCodeFence: false},
			want:   "{\"result\": 42}",
			wantOK: true,
		},
		{
			name:   "newline normalization CRLF and 3+ collapse",
			raw:    "feat: x\r\n\r\n\r\nbody",
			m:      Manifest{Output: "raw", StripCodeFence: true},
			want:   "feat: x\n\nbody",
			wantOK: true,
		},
		{
			name:   "fence-strip-OFF keeps fence",
			raw:    "```\nfeat: x\n```",
			m:      Manifest{Output: "raw", StripCodeFence: false},
			want:   "```\nfeat: x\n```",
			wantOK: true,
		},
		{
			name:   "empty yields false",
			raw:    "",
			m:      Manifest{Output: "raw", StripCodeFence: true},
			want:   "",
			wantOK: false,
		},
		{
			name:   "whitespace-only yields false",
			raw:    "   \n\t  \n",
			m:      Manifest{Output: "raw", StripCodeFence: true},
			want:   "",
			wantOK: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseOutput(tc.raw, tc.m)
			if got != tc.want {
				t.Errorf("parseOutput msg = %q, want %q", got, tc.want)
			}
			if ok != tc.wantOK {
				t.Errorf("parseOutput ok = %v, want %v", ok, tc.wantOK)
			}
		})
	}
}

// TestParseOutput_EmptyOutputIsRaw proves the §12.1/§12.9 default: an empty
// m.Output is RAW, not json. A clean message passes through untouched and ok
// is true — it is NOT routed through the JSON extraction path.
func TestParseOutput_EmptyOutputIsRaw(t *testing.T) {
	m := Manifest{Output: "", JSONField: "result", StripCodeFence: true}
	got, ok := parseOutput("feat: add parser", m)
	if got != "feat: add parser" {
		t.Errorf("empty Output: msg = %q, want %q (empty Output must be raw)", got, "feat: add parser")
	}
	if !ok {
		t.Errorf("empty Output: ok = false, want true")
	}
}

// TestParseOutput_PiFixture is an integration-flavored case reusing the
// realistic pi builtin manifest (Output=raw, StripCodeFence=true) to confirm
// the parser works against a real-world provider configuration, not just
// inline literals.
func TestParseOutput_PiFixture(t *testing.T) {
	m := sixBuiltinManifests()["pi"]
	raw := "```\nfeat: wire provider parser\n```"
	got, ok := parseOutput(raw, m)
	if want := "feat: wire provider parser"; got != want {
		t.Errorf("pi fixture: msg = %q, want %q", got, want)
	}
	if !ok {
		t.Errorf("pi fixture: ok = false, want true")
	}
}
