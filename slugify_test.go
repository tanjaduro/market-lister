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
