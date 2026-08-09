package infra

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/lean-tech/git-rodolfo/internal/domain"
)

// GitHubCLIAdapter shells out to the system's `gh` binary (RF-14).
type GitHubCLIAdapter struct{}

// NewGitHubCLIAdapter returns a GitHubCLIAdapter backed by the system gh.
func NewGitHubCLIAdapter() *GitHubCLIAdapter {
	return &GitHubCLIAdapter{}
}

// ErrGitHubCLINotAvailable is returned when `gh` isn't installed or isn't
// authenticated at all — distinct from being authenticated as the wrong
// account (RF-14's actual mismatch case).
var ErrGitHubCLINotAvailable = fmt.Errorf("GitHub CLI (gh) is not installed or not authenticated")

// ActiveCLIUsername returns the GitHub username `gh` is currently
// authenticated as.
func (g *GitHubCLIAdapter) ActiveCLIUsername() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "gh", "api", "user", "--jq", ".login")
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", ErrGitHubCLINotAvailable
	}
	return strings.TrimSpace(stdout.String()), nil
}

// RegisterPublicKey registers publicKey on GitHub via `gh ssh-key add`.
// Callers must check ActiveCLIUsername against the account first (RF-14):
// this method does not — it registers on whatever account gh is currently
// authenticated as.
func (g *GitHubCLIAdapter) RegisterPublicKey(account domain.Account, publicKey string) error {
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "gh", "ssh-key", "add", "-", "--title", account.DisplayName)
	cmd.Stdin = strings.NewReader(publicKey)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gh ssh-key add: %s", firstLine(stderr.String()))
	}
	return nil
}

// VerifyAuthentication reports whether `gh` itself is authenticated as
// account's provider username, via `gh auth status`. This is independent
// of the SSH-based verification in RF-13 (AuthenticationService), which is
// what Git Rodolfo actually relies on to authorize clone/push.
func (g *GitHubCLIAdapter) VerifyAuthentication(account domain.Account) (domain.AuthResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	out, _ := exec.CommandContext(ctx, "gh", "auth", "status").CombinedOutput()
	return parseGHAuthStatus(string(out), account.ProviderUsername), nil
}

// parseGHAuthStatus is VerifyAuthentication's matching rule as a pure
// function, so it's unit-testable without gh installed or a live session.
func parseGHAuthStatus(output, providerUsername string) domain.AuthResult {
	marker := "Logged in to github.com account " + providerUsername
	if strings.Contains(output, marker) {
		return domain.AuthResult{Success: true, Username: providerUsername, RawOutput: output}
	}
	return domain.AuthResult{Success: false, RawOutput: output}
}
