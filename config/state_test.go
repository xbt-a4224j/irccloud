package config

import (
	"os"
	"path/filepath"
	"testing"
)

// The whole point of the split: saving UI state must not touch the file
// holding the password.
func TestSaveLastChannelDoesNotTouchCredentialFile(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.yaml")
	writeConfig(configFile, Data{Username: "user", Password: "hunter2"})

	before, err := os.Stat(configFile)
	if err != nil {
		t.Fatal(err)
	}

	SaveLastChannel(configFile, "#gonuts")

	// The state must actually have been persisted, otherwise this test
	// passes against an implementation that does nothing at all.
	if got := LoadState(configFile).LastChan; got != "#gonuts" {
		t.Fatalf("LastChan = %q, want %q", got, "#gonuts")
	}

	after, err := os.Stat(configFile)
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) || before.Size() != after.Size() {
		t.Error("credential file was rewritten while saving UI state")
	}
}

// Regression test for -c being honoured on read but ignored on write,
// which copied one account's password into the default config.
func TestSaveLastChannelStaysBesideTheGivenConfig(t *testing.T) {
	work := filepath.Join(t.TempDir(), "work.yaml")
	writeConfig(work, Data{Username: "work@example.invalid", Password: "worksecret"})

	other := filepath.Join(t.TempDir(), "config.yaml")
	writeConfig(other, Data{Username: "personal@example.invalid", Password: "personalsecret"})
	untouched, err := os.ReadFile(other)
	if err != nil {
		t.Fatal(err)
	}

	SaveLastChannel(work, "#work")

	if LoadState(work).LastChan != "#work" {
		t.Error("state was not saved beside the config that was loaded")
	}
	if LoadState(other).LastChan != "" {
		t.Error("state leaked into an unrelated config directory")
	}

	now, err := os.ReadFile(other)
	if err != nil {
		t.Fatal(err)
	}
	if string(untouched) != string(now) {
		t.Error("the unrelated config file was modified")
	}
}

func TestStateFileIsOwnerOnly(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "config.yaml")

	SaveLastChannel(configFile, "#gonuts")

	info, err := os.Stat(statePath(configFile))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("state mode = %04o, want 0600", perm)
	}
}

func TestLoadStateMissingFileIsNotAnError(t *testing.T) {
	if got := LoadState(filepath.Join(t.TempDir(), "config.yaml")); got.LastChan != "" {
		t.Errorf("LastChan = %q, want empty", got.LastChan)
	}
}

// Existing configs carry last_chan inline. The state file wins once it
// exists, but an untouched config must still restore the user's channel.
func TestResolveLastChannelPrefersStateThenConfig(t *testing.T) {
	cases := []struct {
		name       string
		configChan string
		stateChan  string
		want       string
	}{
		{"state wins over config", "#old", "#new", "#new"},
		{"falls back to legacy config value", "#old", "", "#old"},
		{"neither set", "", "", ""},
		{"state only", "", "#new", "#new"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			configFile := filepath.Join(t.TempDir(), "config.yaml")
			data := Data{Username: "u", Password: "p", LastChan: tc.configChan}
			writeConfig(configFile, data)
			SaveLastChannel(configFile, tc.stateChan)

			if got := ResolveLastChannel(configFile, data); got != tc.want {
				t.Errorf("ResolveLastChannel = %q, want %q", got, tc.want)
			}
		})
	}
}
