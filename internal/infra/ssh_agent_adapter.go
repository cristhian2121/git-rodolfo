package infra

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// SSHAgentAdapter shells out to ssh-add to inspect and load keys into a
// running ssh-agent. It never reads a passphrase: AddKey connects
// ssh-add's stdio directly to the terminal, so ssh-add itself prompts the
// user (RF-11).
type SSHAgentAdapter struct{}

// NewSSHAgentAdapter returns an SSHAgentAdapter backed by the system ssh-add.
func NewSSHAgentAdapter() *SSHAgentAdapter {
	return &SSHAgentAdapter{}
}

// IsRunning reports whether a usable ssh-agent is reachable. `ssh-add -l`
// exits 0 (has keys) or 1 ("no identities") when the agent is running, and
// 2 when it can't connect to one at all.
func (a *SSHAgentAdapter) IsRunning() bool {
	if os.Getenv("SSH_AUTH_SOCK") == "" {
		return false
	}
	cmd := exec.Command("ssh-add", "-l")
	err := cmd.Run()
	if err == nil {
		return true
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode() == 1
	}
	return false
}

// IsKeyLoaded reports whether keyPath's key is already loaded in the agent,
// comparing fingerprints so it works regardless of the comment ssh-add
// displays.
func (a *SSHAgentAdapter) IsKeyLoaded(keyPath string) (bool, error) {
	want, err := (&SSHClient{}).PublicKeyFingerprint(keyPath)
	if err != nil {
		return false, err
	}

	var stdout bytes.Buffer
	cmd := exec.Command("ssh-add", "-l")
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return false, nil // agent running, no identities loaded
		}
		return false, fmt.Errorf("ssh-add -l: %w", err)
	}

	for _, line := range strings.Split(stdout.String(), "\n") {
		fields := strings.Fields(line)
		for _, f := range fields {
			if f == want {
				return true, nil
			}
		}
	}
	return false, nil
}

// AddKey loads keyPath into the agent (RF-11), optionally also storing the
// passphrase in the macOS Keychain via --apple-use-keychain. That flag is
// only meaningful on macOS.
func (a *SSHAgentAdapter) AddKey(keyPath string, useKeychain bool) error {
	args := []string{}
	if useKeychain {
		if runtime.GOOS != "darwin" {
			return fmt.Errorf("Keychain integration is only available on macOS")
		}
		args = append(args, "--apple-use-keychain")
	}
	args = append(args, keyPath)

	cmd := exec.Command("ssh-add", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ssh-add: %w", err)
	}
	return nil
}
