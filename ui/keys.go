package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell"
)

// DefaultChannelPickerKey is used when the config does not set one.
// Ctrl+Space is the historical binding but macOS claims it for switching
// input sources, so it never reaches the application there.
const DefaultChannelPickerKey = "ctrl+g"

var ctrlKeys = map[rune]tcell.Key{
	'a': tcell.KeyCtrlA, 'b': tcell.KeyCtrlB, 'c': tcell.KeyCtrlC,
	'd': tcell.KeyCtrlD, 'e': tcell.KeyCtrlE, 'f': tcell.KeyCtrlF,
	'g': tcell.KeyCtrlG, 'h': tcell.KeyCtrlH, 'i': tcell.KeyCtrlI,
	'j': tcell.KeyCtrlJ, 'k': tcell.KeyCtrlK, 'l': tcell.KeyCtrlL,
	'm': tcell.KeyCtrlM, 'n': tcell.KeyCtrlN, 'o': tcell.KeyCtrlO,
	'p': tcell.KeyCtrlP, 'q': tcell.KeyCtrlQ, 'r': tcell.KeyCtrlR,
	's': tcell.KeyCtrlS, 't': tcell.KeyCtrlT, 'u': tcell.KeyCtrlU,
	'v': tcell.KeyCtrlV, 'w': tcell.KeyCtrlW, 'x': tcell.KeyCtrlX,
	'y': tcell.KeyCtrlY, 'z': tcell.KeyCtrlZ,
}

// Swallowed by the terminal driver or the shell before the application
// sees them. Binding one produces a key that simply never fires.
var reservedKeys = map[rune]string{
	'c': "sends SIGINT",
	'd': "signals end of input",
	'q': "resumes terminal output (flow control)",
	's': "pauses terminal output (flow control)",
	'z': "suspends the process",

	// These control codes are literally other keys at the terminal level,
	// so binding one would hijack ordinary typing.
	'i': "is Tab",
	'm': "is Enter",
	'h': "is Backspace",
	'j': "is newline",
}

// Already bound elsewhere in the client; rebinding would shadow them.
var boundKeys = map[rune]string{
	'a': "move to start of line",
	'b': "jump to most recent activity",
	'e': "move to end of line",
	'k': "delete to end of line",
	'u': "delete the line",
	'w': "delete the previous word",
}

// ParseKey turns a config string such as "ctrl+g" into a tcell key.
// Accepts ctrl+g, ctrl-g, ctrl g, c-g and ^g, case-insensitively.
func ParseKey(name string) (tcell.Key, error) {
	s := strings.ToLower(strings.TrimSpace(name))

	if s == "" {
		return 0, fmt.Errorf("no key given")
	}

	if s == "ctrl+space" || s == "ctrl-space" || s == "ctrl space" {
		return tcell.KeyCtrlSpace, nil
	}

	for _, prefix := range []string{"ctrl+", "ctrl-", "ctrl ", "c-", "^"} {
		if rest, ok := strings.CutPrefix(s, prefix); ok {
			return parseCtrlLetter(rest, name)
		}
	}

	return 0, fmt.Errorf("unrecognised key %q: use a form like ctrl+g", name)
}

func parseCtrlLetter(rest, original string) (tcell.Key, error) {
	runes := []rune(rest)

	if len(runes) != 1 {
		return 0, fmt.Errorf("unrecognised key %q: expected a single letter after ctrl", original)
	}

	letter := runes[0]

	key, ok := ctrlKeys[letter]
	if !ok {
		return 0, fmt.Errorf("unrecognised key %q: ctrl+%c is not a control key", original, letter)
	}

	if why, taken := reservedKeys[letter]; taken {
		return 0, fmt.Errorf("ctrl+%c cannot be used: it %s. Try one of: %s",
			letter, why, suggestions())
	}

	if why, taken := boundKeys[letter]; taken {
		return 0, fmt.Errorf("ctrl+%c is already bound to %q. Try one of: %s",
			letter, why, suggestions())
	}

	return key, nil
}

// suggestions lists control keys that are neither reserved nor bound.
func suggestions() string {
	var free []string

	for letter := range ctrlKeys {
		if _, taken := reservedKeys[letter]; taken {
			continue
		}
		if _, taken := boundKeys[letter]; taken {
			continue
		}
		free = append(free, fmt.Sprintf("ctrl+%c", letter))
	}

	sort.Strings(free)

	if len(free) > 8 {
		free = free[:8]
	}

	return strings.Join(free, ", ")
}
