package ui

import (
	"testing"

	"github.com/gdamore/tcell"
)

func TestParseKeyAcceptsCommonSpellings(t *testing.T) {
	cases := []struct {
		in   string
		want tcell.Key
	}{
		{"ctrl+g", tcell.KeyCtrlG},
		{"Ctrl+G", tcell.KeyCtrlG},
		{"CTRL-G", tcell.KeyCtrlG},
		{"ctrl g", tcell.KeyCtrlG},
		{"^g", tcell.KeyCtrlG},
		{"c-g", tcell.KeyCtrlG},
		{"ctrl+o", tcell.KeyCtrlO},
		{"ctrl+space", tcell.KeyCtrlSpace},
		{"  ctrl+t  ", tcell.KeyCtrlT},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseKey(tc.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("ParseKey(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseKeyRejectsUnusableBindings(t *testing.T) {
	// These are swallowed by the terminal or the shell before the
	// application ever sees them, so accepting them would silently
	// produce a dead keybinding.
	for _, in := range []string{"ctrl+c", "ctrl+z", "ctrl+s", "ctrl+q", "ctrl+d"} {
		t.Run(in, func(t *testing.T) {
			if _, err := ParseKey(in); err == nil {
				t.Errorf("ParseKey(%q) should be rejected as terminal-reserved", in)
			}
		})
	}
}

func TestParseKeyRejectsGarbage(t *testing.T) {
	for _, in := range []string{"", "banana", "ctrl+", "alt+g", "f13", "ctrl+gg"} {
		t.Run(in, func(t *testing.T) {
			if _, err := ParseKey(in); err == nil {
				t.Errorf("ParseKey(%q) should have failed", in)
			}
		})
	}
}

// A binding that collides with an existing one would shadow it.
func TestParseKeyRejectsKeysAlreadyBound(t *testing.T) {
	for _, in := range []string{"ctrl+b", "ctrl+a", "ctrl+e", "ctrl+k", "ctrl+w", "ctrl+u"} {
		t.Run(in, func(t *testing.T) {
			if _, err := ParseKey(in); err == nil {
				t.Errorf("ParseKey(%q) should be rejected as already bound", in)
			}
		})
	}
}

// In a terminal these control codes ARE Tab, Enter, Backspace and
// newline. Binding one would hijack ordinary typing.
func TestParseKeyRejectsKeysThatAreOtherKeys(t *testing.T) {
	for _, in := range []string{"ctrl+i", "ctrl+m", "ctrl+h", "ctrl+j"} {
		t.Run(in, func(t *testing.T) {
			if _, err := ParseKey(in); err == nil {
				t.Errorf("ParseKey(%q) should be rejected: it is indistinguishable from another key", in)
			}
		})
	}
}

// Whatever the default is, it must actually be usable.
func TestDefaultChannelPickerKeyIsValid(t *testing.T) {
	if _, err := ParseKey(DefaultChannelPickerKey); err != nil {
		t.Errorf("default binding %q is not valid: %v", DefaultChannelPickerKey, err)
	}
}

func TestResolvePickerKeyFallsBack(t *testing.T) {
	fallback, _ := ParseKey(DefaultChannelPickerKey)

	cases := []struct {
		name       string
		configured string
		want       tcell.Key
	}{
		{"empty uses the default", "", fallback},
		{"whitespace uses the default", "   ", fallback},
		{"a valid key is honoured", "ctrl+o", tcell.KeyCtrlO},
		{"an unusable key falls back", "ctrl+c", fallback},
		{"garbage falls back", "banana", fallback},
		{"an already-bound key falls back", "ctrl+b", fallback},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolvePickerKey(tc.configured); got != tc.want {
				t.Errorf("resolvePickerKey(%q) = %v, want %v", tc.configured, got, tc.want)
			}
		})
	}
}
