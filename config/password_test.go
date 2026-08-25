package config

import (
	"strings"
	"testing"
)

func TestResolvePasswordUsesInlinePasswordWhenNoCommandIsSet(t *testing.T) {
	got, err := Data{Username: "u", Password: "hunter2"}.ResolvePassword()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hunter2" {
		t.Errorf("ResolvePassword() = %q, want %q", got, "hunter2")
	}
}

func TestResolvePasswordRunsPasswordCommand(t *testing.T) {
	got, err := Data{Username: "u", PasswordCommand: "printf %s from-secret-store"}.ResolvePassword()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "from-secret-store" {
		t.Errorf("ResolvePassword() = %q, want %q", got, "from-secret-store")
	}
}

// Secret managers almost always emit a trailing newline.
func TestResolvePasswordTrimsTrailingNewline(t *testing.T) {
	got, err := Data{Username: "u", PasswordCommand: "echo hunter2"}.ResolvePassword()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hunter2" {
		t.Errorf("ResolvePassword() = %q, want %q", got, "hunter2")
	}
}

// A password that legitimately contains spaces must survive intact.
func TestResolvePasswordPreservesInternalWhitespace(t *testing.T) {
	got, err := Data{Username: "u", PasswordCommand: "printf %s 'correct horse battery staple'"}.ResolvePassword()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "correct horse battery staple" {
		t.Errorf("ResolvePassword() = %q, want the passphrase intact", got)
	}
}

func TestResolvePasswordPrefersCommandOverInlinePassword(t *testing.T) {
	got, err := Data{Username: "u", Password: "stale", PasswordCommand: "printf %s fresh"}.ResolvePassword()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "fresh" {
		t.Errorf("ResolvePassword() = %q, want %q", got, "fresh")
	}
}

func TestResolvePasswordReportsCommandFailure(t *testing.T) {
	_, err := Data{Username: "u", PasswordCommand: "exit 7"}.ResolvePassword()
	if err == nil {
		t.Fatal("expected an error when the password command fails")
	}
	if !strings.Contains(err.Error(), "password_command") {
		t.Errorf("error = %q, want it to mention password_command", err)
	}
}

func TestResolvePasswordRejectsEmptyCommandOutput(t *testing.T) {
	_, err := Data{Username: "u", PasswordCommand: "true"}.ResolvePassword()
	if err == nil {
		t.Fatal("expected an error when the password command produces nothing")
	}
}

// The command's stderr must not be mistaken for the password.
func TestResolvePasswordIgnoresStderr(t *testing.T) {
	got, err := Data{Username: "u", PasswordCommand: "echo noise >&2; printf %s realsecret"}.ResolvePassword()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "realsecret" {
		t.Errorf("ResolvePassword() = %q, want %q", got, "realsecret")
	}
}

// A config using password_command has no inline password, so it must not
// be mistaken for the untouched skeleton.
func TestIsPlaceholderAcceptsPasswordCommand(t *testing.T) {
	d := Data{Username: "real@example.invalid", PasswordCommand: "printf %s x"}
	if d.IsPlaceholder() {
		t.Error("a config using password_command is not a placeholder config")
	}
}

// Nobody will discover password_command unless the generated config
// mentions it.
func TestSkeletonDocumentsPasswordCommand(t *testing.T) {
	filename := tempConfig(t)

	parseData(filename)

	content := readFile(t, filename)
	if !strings.Contains(content, "password_command") {
		t.Error("skeleton config does not mention password_command")
	}

	// It must be commented out, or it would run on the next launch.
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "password_command:") {
			t.Errorf("password_command is active in the skeleton: %q", line)
		}
	}
}

// The skeleton must still be valid YAML that parses back to placeholders.
func TestSkeletonStillRoundTrips(t *testing.T) {
	filename := tempConfig(t)

	written := parseData(filename)
	reread := parseData(filename)

	if written.Username != reread.Username || written.Password != reread.Password {
		t.Errorf("skeleton did not round-trip: %+v vs %+v", written, reread)
	}
	if !reread.IsPlaceholder() {
		t.Error("re-read skeleton should still count as a placeholder config")
	}
}
