package git

import (
	"strings"
	"testing"
)

// TestEstimateTokens pins the FR3d/FR3i estimator contract: ceil(runeCount/4), rune-based, ceiling (not
// truncating). Expectations are HARDCODED (never derived from the function — that would be circular and
// couldn't catch a wrong formula). Pure table test; no git repo, no I/O.
func TestEstimateTokens(t *testing.T) {
	longASCII := strings.Repeat("a", 4000) // 4000 runes → ceil(4000/4)=1000
	cjk4 := "你好世界"                         // 4 runes, 12 bytes → 1 token (NOT 3 — pins rune-based counting)

	tests := []struct {
		in   string
		want int
		desc string
	}{
		{"", 0, "empty string → 0 tokens"},
		{"a", 1, "1 rune → 1 (ceiling: any non-empty string ≥ 1)"},
		{"abcd", 1, "4 ASCII → 1 (exact multiple)"},
		{"abcde", 2, "5 ASCII → 2 (ceilDiv(5,4)=2 — the ceiling pin)"},
		{"abcdefgh", 2, "8 ASCII → 2"},
		{cjk4, 1, "4-rune/12-byte CJK → 1 (RUNE-based, not byte-based 3)"},
		{longASCII, 1000, "4000-rune string → 1000 (no int overflow)"},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			got := EstimateTokens(tc.in)
			if got != tc.want {
				t.Errorf("EstimateTokens(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

// TestEstimateTokensBytes pins the []byte form: same ceil(runes/4) formula, rune-based, and parity with the
// string form (a string and its []byte conversion estimate identically). Expectations HARDCODED.
func TestEstimateTokensBytes(t *testing.T) {
	tests := []struct {
		in   string // []byte forms derived from these literals for parity
		want int
		desc string
	}{
		{"abcd", 1, "[]byte(\"abcd\") → 1"},
		{"你好世界", 1, "[]byte(CJK 4 runes/12 bytes) → 1 (rune-based parity)"},
		{"abcdefgh", 2, "[]byte(\"abcdefgh\") → 2"},
		{"", 0, "empty []byte → 0"},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			got := EstimateTokensBytes([]byte(tc.in))
			if got != tc.want {
				t.Errorf("EstimateTokensBytes([]byte(%q)) = %d, want %d", tc.in, got, tc.want)
			}
			// Parity: a string and its []byte form must estimate identically (both use ceilDiv(runes,4)).
			if strTokens := EstimateTokens(tc.in); got != strTokens {
				t.Errorf("parity break: EstimateTokensBytes([]byte(%q))=%d != EstimateTokens(%q)=%d",
					tc.in, got, tc.in, strTokens)
			}
		})
	}
}

// TestCeilDiv pins the unexported ceiling-division helper directly: 0 for n=0, ceiling for n>0, exact multiples.
func TestCeilDiv(t *testing.T) {
	tests := []struct {
		n, d, want int
		desc       string
	}{
		{0, 4, 0, "0/4 → 0 (no special-case)"},
		{1, 4, 1, "1/4 → 1 (ceiling)"},
		{4, 4, 1, "4/4 → 1 (exact)"},
		{5, 4, 2, "5/4 → 2 (ceiling)"},
		{8, 4, 2, "8/4 → 2 (exact)"},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			got := ceilDiv(tc.n, tc.d)
			if got != tc.want {
				t.Errorf("ceilDiv(%d, %d) = %d, want %d", tc.n, tc.d, got, tc.want)
			}
		})
	}
}
