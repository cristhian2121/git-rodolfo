package infra

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// EnvironmentInspector inspects the real local system (§13.10 #1, #8).
type EnvironmentInspector struct{}

// NewEnvironmentInspector returns an EnvironmentInspector.
func NewEnvironmentInspector() *EnvironmentInspector {
	return &EnvironmentInspector{}
}

// GitVersion runs `git --version` and extracts the version number.
func (e *EnvironmentInspector) GitVersion() (string, error) {
	var stdout bytes.Buffer
	cmd := exec.Command("git", "--version")
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git not found: %w", err)
	}
	// Typical output: "git version 2.43.0"
	fields := strings.Fields(stdout.String())
	if len(fields) < 3 {
		return "", fmt.Errorf("could not parse git version from %q", stdout.String())
	}
	return fields[2], nil
}

// SSHInstalled reports whether the ssh binary is on PATH.
func (e *EnvironmentInspector) SSHInstalled() bool {
	_, err := exec.LookPath("ssh")
	return err == nil
}

func (e *EnvironmentInspector) Getenv(key string) string {
	return os.Getenv(key)
}
