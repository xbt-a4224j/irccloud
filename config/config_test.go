package config

import (
	"os"
	"path/filepath"
	"testing"
)

// Regression test for the filepath.Base/filepath.Dir mixup: parsing a
// config whose directory does not exist must create that directory, not a
// directory named after the file in the current working directory.
func TestParseDataCreatesParentDirectory(t *testing.T) {
	root := t.TempDir()
	filename := filepath.Join(root, "nested", "irccloud", "config.yaml")

	// Run from a scratch cwd so we can detect a stray directory being
	// created relative to it.
	cwd := t.TempDir()
	restore := chdir(t, cwd)
	defer restore()

	parseData(filename)

	info, err := os.Stat(filepath.Dir(filename))
	if err != nil {
		t.Fatalf("parent directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected %s to be a directory", filepath.Dir(filename))
	}

	if _, err := os.Stat(filepath.Join(cwd, "config.yaml")); err == nil {
		t.Fatal("a stray 'config.yaml' was created in the working directory")
	}
}

func TestParseDataWritesSkeletonConfig(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "sub", "config.yaml")

	got := parseData(filename)

	if _, err := os.Stat(filename); err != nil {
		t.Fatalf("skeleton config was not written: %v", err)
	}
	if got.Username != placeholderUsername {
		t.Errorf("Username = %q, want %q", got.Username, placeholderUsername)
	}
	if got.Password != placeholderPassword {
		t.Errorf("Password = %q, want %q", got.Password, placeholderPassword)
	}
}

// The credential file must never be created group- or world-readable.
func TestSkeletonConfigIsOwnerOnly(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "sub", "config.yaml")

	parseData(filename)

	info, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("config mode = %04o, want 0600", perm)
	}

	dir, err := os.Stat(filepath.Dir(filename))
	if err != nil {
		t.Fatal(err)
	}
	if perm := dir.Mode().Perm(); perm != 0700 {
		t.Errorf("config dir mode = %04o, want 0700", perm)
	}
}

func TestRoundTripPreservesFields(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "config.yaml")
	want := Data{
		Username:     "user@example.invalid",
		Password:     "hunter2",
		Triggers:     []string{"cakes", "nick"},
		LastChan:     "#gonuts",
		OnlyMessages: true,
	}

	writeConfig(filename, want)
	got := parseData(filename)

	if got.Username != want.Username || got.Password != want.Password {
		t.Errorf("credentials did not round-trip: %+v", got)
	}
	if got.LastChan != want.LastChan || got.OnlyMessages != want.OnlyMessages {
		t.Errorf("settings did not round-trip: %+v", got)
	}
	if len(got.Triggers) != len(want.Triggers) {
		t.Errorf("Triggers = %v, want %v", got.Triggers, want.Triggers)
	}
}

func chdir(t *testing.T, dir string) func() {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return func() { _ = os.Chdir(prev) }
}

// The core of the permission bug: os.WriteFile's perm argument is ignored
// when the file already exists, so writing over a 0644 config used to leave
// the password world-readable.
func TestWriteConfigTightensExistingLoosePermissions(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "config.yaml")

	if err := os.WriteFile(filename, []byte("username: old\n"), 0644); err != nil {
		t.Fatal(err)
	}

	writeConfig(filename, Data{Username: "user", Password: "hunter2"})

	info, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("mode after write = %04o, want 0600 (password left readable)", perm)
	}
}

// A failed write must not destroy the existing credentials.
func TestWriteConfigLeavesNoTempFilesBehind(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "config.yaml")

	writeConfig(filename, Data{Username: "user", Password: "hunter2"})

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "config.yaml" {
			t.Errorf("stray file left behind: %s", e.Name())
		}
	}
}

func TestIsPlaceholder(t *testing.T) {
	cases := []struct {
		name string
		data Data
		want bool
	}{
		{"skeleton", Data{Username: placeholderUsername, Password: placeholderPassword}, true},
		{"empty", Data{}, true},
		{"password left as placeholder", Data{Username: "real@example.invalid", Password: placeholderPassword}, true},
		{"filled in", Data{Username: "real@example.invalid", Password: "hunter2"}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.data.IsPlaceholder(); got != tc.want {
				t.Errorf("IsPlaceholder() = %v, want %v", got, tc.want)
			}
		})
	}
}

func tempConfig(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "sub", "config.yaml")
}

func readFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
