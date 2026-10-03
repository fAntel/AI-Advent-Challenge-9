package main

import (
	"strings"
	"testing"
)

func TestSpeakerStyles(t *testing.T) {
	for _, test := range []struct{ label, color, want string }{
		{"You:", "36", "\x1b[1;36mYou:\x1b[0m"},
		{"Agent:", "35", "\x1b[1;35mAgent:\x1b[0m"},
		{"Execute? [y/N]", "33", "\x1b[1;33mExecute? [y/N]\x1b[0m"},
	} {
		if got := styleLabel(test.label, test.color, true, "xterm", false); got != test.want {
			t.Fatalf("%q: %q", test.label, got)
		}
		for _, plain := range []string{styleLabel(test.label, test.color, false, "xterm", false), styleLabel(test.label, test.color, true, "dumb", false), styleLabel(test.label, test.color, true, "xterm", true)} {
			if plain != test.label || strings.Contains(plain, "\x1b") {
				t.Fatalf("expected plain %q, got %q", test.label, plain)
			}
		}
	}
}
