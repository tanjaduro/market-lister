package main

import "testing"

func TestSlugify(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"simple spaces", "reima pink 24", "reima-pink-24"},
		{"ampersand stripped and dashes collapse", "Nike Air & Jordan", "nike-air-jordan"},
		{"umlauts dropped without transliteration", "Größe 24", "gre-24"},
		{"leading and trailing whitespace trimmed", "  reima pink 24  ", "reima-pink-24"},
		{"consecutive specials collapse to single dash", "a !@# b", "a-b"},
		{"empty string", "", ""},
		{"only dashes collapse and trim to empty", "---", ""},
		{"only non-ASCII letters drop to empty", "中文", ""},
		{"mixed ASCII and accent drops the accent", "café au lait", "caf-au-lait"},
		{"all emoji drops to empty", "🎉🎉🎉", ""},
		{"already clean lowercase passes through", "already-clean", "already-clean"},
		{"trailing hyphen trimmed", "trailing-", "trailing"},
		{"leading hyphen trimmed", "-leading", "leading"},
		{"double underscore collapses to single dash", "a__b", "a-b"},
		{"number prefix preserved", "9 lives", "9-lives"},
		{"mixed case lowercased", "MixedCase Foo", "mixedcase-foo"},
		{"dot dropped from filename-style input", "thing v2.0", "thing-v20"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slugify(tt.in)
			if got != tt.want {
				t.Errorf("slugify(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
