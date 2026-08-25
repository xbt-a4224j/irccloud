package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

// State holds volatile UI preferences. It lives in its own file so that
// persisting something as trivial as the last viewed channel never
// requires rewriting the file that holds the user's credentials.
type State struct {
	LastChan string `yaml:"last_chan"`
}

func statePath(configFile string) string {
	return filepath.Join(filepath.Dir(configFile), "state.yaml")
}

// LoadState reads the state file beside the given config. A missing or
// unreadable state file is not an error; the zero value is returned.
func LoadState(configFile string) State {
	var s State

	content, err := os.ReadFile(statePath(configFile))
	if err != nil {
		return s
	}

	_ = yaml.Unmarshal(content, &s)

	return s
}

// SaveLastChannel records the last viewed channel beside the config file
// that was actually loaded, leaving the credential file untouched.
func SaveLastChannel(configFile, latest string) {
	if latest == "" {
		return
	}

	path := statePath(configFile)

	content, err := yaml.Marshal(&State{LastChan: latest})
	if err != nil {
		return
	}

	if err := os.MkdirAll(filepath.Dir(path), configDirMode); err != nil {
		fmt.Printf("Could not create config directory %s: %v\n", filepath.Dir(path), err)
		return
	}

	if err := os.WriteFile(path, content, configFileMode); err != nil {
		fmt.Printf("Could not write state to file %s: %v\n", path, err)
	}
}

// ResolveLastChannel reports which channel to open on startup. The state
// file takes precedence; the config's inline last_chan is honoured as a
// fallback so existing configs keep working.
func ResolveLastChannel(configFile string, data Data) string {
	if chosen := LoadState(configFile).LastChan; chosen != "" {
		return chosen
	}

	return data.LastChan
}

// LogPath is the diagnostics log beside the given config file.
func LogPath(configFile string) string {
	return filepath.Join(filepath.Dir(configFile), "irccloud.log")
}

// OpenLog opens the diagnostics log for appending. Log output must not go
// to stderr while the TUI owns the terminal, or it paints over the chat.
func OpenLog(configFile string) (*os.File, error) {
	path := LogPath(configFile)

	if err := os.MkdirAll(filepath.Dir(path), configDirMode); err != nil {
		return nil, err
	}

	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, configFileMode)
}
