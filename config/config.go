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
	Username     string   `yaml:"username"`
	Password     string   `yaml:"password"`
	Triggers     []string `yaml:"triggers"`
	LastChan     string   `yaml:"last_chan"`
	OnlyMessages bool     `yaml:"only_messages"`
}

// IsPlaceholder reports whether the config still holds the generated
// skeleton values, meaning the user has not filled in their credentials.
func (d Data) IsPlaceholder() bool {
	return d.Username == "" || d.Password == "" ||
		d.Username == placeholderUsername || d.Password == placeholderPassword
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

func writeDummyConfig(filename string) Data {
	dummy := Data{
		Username:     placeholderUsername,
		Password:     placeholderPassword,
		Triggers:     []string{},
		LastChan:     "",
		OnlyMessages: false,
	}

	writeConfig(filename, dummy)
	return dummy
}

// DefaultPath is the config location used when -c is not supplied.
func DefaultPath() string {
	filename, _ := getPaths()
	return filename
}
