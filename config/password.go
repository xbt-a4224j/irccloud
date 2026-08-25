package config

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// ResolvePassword reports the password to authenticate with.
//
// When password_command is set it is run through the shell and its stdout
// is used, so the credential can live in a secret manager instead of on
// disk in cleartext. This is the same mechanism msmtp, isync and aerc use.
// Only stdout is consumed; stderr is left alone so the command can prompt
// or warn without corrupting the secret.
func (d Data) ResolvePassword() (string, error) {
	if d.PasswordCommand == "" {
		return d.Password, nil
	}

	cmd := exec.Command("sh", "-c", d.PasswordCommand)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("password_command failed: %w", err)
	}

	// Secret managers conventionally emit a trailing newline. Trim only
	// that, so a passphrase containing spaces survives intact.
	password := strings.TrimRight(stdout.String(), "\r\n")

	if password == "" {
		return "", fmt.Errorf("password_command produced no output")
	}

	return password, nil
}
