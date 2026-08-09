package infra

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/lean-tech/git-rodolfo/internal/app"
)

// GitClient shells out to the system git binary, scoped to one working
// directory (via `git -C dir`) — one instance per repository, matching how
// app.GitClient is used.
type GitClient struct {
	dir string
}

// NewGitClient returns a GitClient bound to dir.
func NewGitClient(dir string) *GitClient {
	return &GitClient{dir: dir}
}

// IsRepository reports whether path is inside a Git working tree. Unlike
// the other methods, it operates on the given path directly rather than
// the bound directory (see the app.GitClient doc).
func (g *GitClient) IsRepository(path string) bool {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--is-inside-work-tree")
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

// CloneWithSSHCommand clones remoteURL into destination, forcing sshCommand
// from the very first connection (§12.2) unless it's empty.
func (g *GitClient) CloneWithSSHCommand(remoteURL, destination, sshCommand string) error {
	args := []string{}
	if sshCommand != "" {
		args = append(args, "-c", "core.sshCommand="+sshCommand)
	}
	// "--" stops git from ever interpreting remoteURL/destination as
	// options — otherwise a destination (or, in principle, a URL) that
	// happens to start with "-" could be parsed as a flag like
	// "--template=<attacker-controlled-dir>", whose hooks would then run
	// during checkout (a known git argument-injection class).
	args = append(args, "clone", "--", remoteURL, destination)

	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stdout = os.Stdout
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
	if err := cmd.Run(); err != nil {
		return &app.GitCommandError{Output: strings.TrimSpace(stderr.String()), Err: err}
	}
	return nil
}

func (g *GitClient) SetLocalConfig(key, value string) error {
	var stderr bytes.Buffer
	cmd := exec.Command("git", "-C", g.dir, "config", "--local", key, value)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git config %s: %s", key, firstLine(stderr.String()))
	}
	return nil
}

func (g *GitClient) GetLocalConfig(key string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("git", "-C", g.dir, "config", "--local", "--get", key)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return "", app.ErrConfigKeyNotSet
		}
		return "", fmt.Errorf("git config --get %s: %s", key, firstLine(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (g *GitClient) GetEffectiveConfig(key string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("git", "-C", g.dir, "config", "--get", key)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return "", app.ErrConfigKeyNotSet
		}
		return "", fmt.Errorf("git config --get %s: %s", key, firstLine(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (g *GitClient) UnsetLocalConfig(key string) error {
	var stderr bytes.Buffer
	cmd := exec.Command("git", "-C", g.dir, "config", "--local", "--unset", key)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		// Exit 5: the key was never set. Unsetting an absent key is a
		// no-op success — Clear must be idempotent (RF-20).
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 5 {
			return nil
		}
		return fmt.Errorf("git config --unset %s: %s", key, firstLine(stderr.String()))
	}
	return nil
}

func (g *GitClient) GetRemoteURL(remote string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("git", "-C", g.dir, "remote", "get-url", remote)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "No such remote") {
			return "", app.ErrNoRemote
		}
		return "", fmt.Errorf("git remote get-url %s: %s", remote, firstLine(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}
