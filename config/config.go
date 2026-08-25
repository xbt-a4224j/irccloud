package config

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

const (
	placeholderUsername = "your_username_here"
	placeholderPassword = "secret_password_here"

	// The config holds the account password in cleartext, so it must never
	// be readable by anyone but the owner.
	configFileMode = 0600
	configDirMode  = 0700
)

type Data struct {
	Username        string   `yaml:"username"`
	Password        string   `yaml:"password"`
	PasswordCommand string   `yaml:"password_command,omitempty"`
	Triggers        []string `yaml:"triggers"`
	LastChan        string   `yaml:"last_chan"`
	OnlyMessages    bool     `yaml:"only_messages"`

	// Empty means the built-in default. Ctrl+Space always works too.
	ChannelPickerKey string `yaml:"channel_picker_key,omitempty"`
}

// IsPlaceholder reports whether the config still holds the generated
// skeleton values, meaning the user has not filled in their credentials.
func (d Data) IsPlaceholder() bool {
	if d.Username == "" || d.Username == placeholderUsername {
		return true
	}

	if d.PasswordCommand != "" {
		return false
	}

	return d.Password == "" || d.Password == placeholderPassword
}

func ParseCustom(filename string) Data {
	return parseData(filename)
}

func Parse() Data {
	filename, _ := getPaths()
	return parseData(filename)
}

func parseData(filename string) Data {
	var result Data

	if err := os.MkdirAll(filepath.Dir(filename), configDirMode); err != nil {
		fmt.Printf("Could not create config directory %s: %v\n", filepath.Dir(filename), err)
		return result
	}

	f, err := os.Open(filename)
	if err != nil {
		fmt.Printf("Could not find config, creating one in %s\n", filename)
		return writeDummyConfig(filename)
	}
	defer f.Close()

	if err := checkPermissions(filename); err != nil {
		fmt.Printf("Warning: %v\n", err)
	}

	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(&result); err != nil {
		panic("Invalid config format!")
	}

	return result
}

// checkPermissions warns when the credential file is readable by anyone
// other than its owner. This catches a config the client did not create
// itself, which writeConfig alone cannot fix.
func checkPermissions(filename string) error {
	info, err := os.Stat(filename)
	if err != nil {
		return nil
	}

	if perm := info.Mode().Perm(); perm&0077 != 0 {
		return fmt.Errorf("%s is group/world readable (mode %04o) and holds your password; run: chmod 600 %s",
			filename, perm, filename)
	}

	return nil
}

func getPaths() (string, string) {
	currUser, _ := user.Current()
	confDir := filepath.Join(currUser.HomeDir, ".config", "irccloud")
	return filepath.Join(confDir, "config.yaml"), confDir
}

// writeConfig writes the config atomically and enforces owner-only
// permissions. The mode passed to OpenFile applies only when a file is
// created, so an existing loose file is fixed by writing a fresh temp file
// and renaming it into place.
func writeConfig(filename string, data Data) {
	content, err := yaml.Marshal(&data)
	if err != nil {
		fmt.Printf("Could not serialise config: %v\n", err)
		return
	}

	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, configDirMode); err != nil {
		fmt.Printf("Could not create config directory %s: %v\n", dir, err)
		return
	}

	// Same directory as the target so the rename stays on one filesystem.
	tmp, err := os.CreateTemp(dir, ".config-*.yaml")
	if err != nil {
		fmt.Printf("Could not write config to file %s: %v\n", filename, err)
		return
	}
	tmpName := tmp.Name()

	defer func() {
		// No-op once the rename has succeeded.
		_ = os.Remove(tmpName)
	}()

	if err := tmp.Chmod(configFileMode); err != nil {
		tmp.Close()
		fmt.Printf("Could not set permissions on config: %v\n", err)
		return
	}

	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		fmt.Printf("Could not write config to file %s: %v\n", filename, err)
		return
	}

	if err := tmp.Close(); err != nil {
		fmt.Printf("Could not write config to file %s: %v\n", filename, err)
		return
	}

	if err := os.Rename(tmpName, filename); err != nil {
		fmt.Printf("Could not write config to file %s: %v\n", filename, err)
		return
	}

	// Rename preserves the temp file's mode, but an existing target may
	// have been replaced on a system where that is not guaranteed.
	if err := os.Chmod(filename, configFileMode); err != nil {
		fmt.Printf("Could not set permissions on config: %v\n", err)
	}
}

// skeletonConfig is written as a literal rather than marshalled so it can
// carry comments explaining password_command, which is the only way a user
// will discover it.
const skeletonConfig = `# IRCCloud terminal client configuration.
# This file holds your credentials. Keep it mode 0600.

username: ` + placeholderUsername + `

# Your IRCCloud password, in cleartext. Prefer password_command below.
password: ` + placeholderPassword + `

# Instead of storing the password here, have it read from a secret manager
# at startup. If set, this takes precedence over "password" above and the
# password never touches disk. The command runs through the shell and its
# stdout is used, with a trailing newline stripped. Examples:
#
#   password_command: security find-generic-password -s irccloud -w
#   password_command: pass show irccloud
#   password_command: op read "op://Personal/IRCCloud/password"

# Words that trigger a notification when mentioned in a channel.
triggers: []

# Set to true to hide join/part/quit noise from chat buffers.
only_messages: false

# Key that opens the channel picker. Ctrl+Space also always works, but
# macOS claims it for switching input sources so it may never reach this
# client. Accepts forms like "ctrl+g", "c-g" or "^g".
channel_picker_key: ctrl+g
`

func writeDummyConfig(filename string) Data {
	if err := writeSkeleton(filename); err != nil {
		fmt.Printf("Could not write config to file %s: %v\n", filename, err)
	}

	return Data{
		Username:     placeholderUsername,
		Password:     placeholderPassword,
		Triggers:     []string{},
		LastChan:     "",
		OnlyMessages: false,
	}
}

func writeSkeleton(filename string) error {
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, configDirMode); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".config-*.yaml")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if err := tmp.Chmod(configFileMode); err != nil {
		tmp.Close()
		return err
	}

	if _, err := tmp.WriteString(skeletonConfig); err != nil {
		tmp.Close()
		return err
	}

	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, filename)
}

// DefaultPath is the config location used when -c is not supplied.
func DefaultPath() string {
	filename, _ := getPaths()
	return filename
}
