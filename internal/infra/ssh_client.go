package infra

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/domain"
)

// connectTimeout bounds network operations per RNF-03.
const connectTimeout = 10 * time.Second

// SSHClient shells out to the system's ssh-keygen and ssh binaries.
//
// It relies on one property of the modern OpenSSH private key format: the
// public key is stored in cleartext alongside the encrypted private
// portion. That means `ssh-keygen -l -f <private-key>` can report a key's
// fingerprint without ever needing its passphrase, which is what lets
// ValidateKey and PublicKeyFingerprint work non-interactively even on a
// key Git Rodolfo has never seen unlocked (RF-11: it must never read,
// request or store a passphrase).
type SSHClient struct{}

// NewSSHClient returns an SSHClient backed by the system's ssh-keygen/ssh.
func NewSSHClient() *SSHClient {
	return &SSHClient{}
}

// GenerateKey creates a new Ed25519 key pair (RF-06). When withPassphrase
// is true, ssh-keygen's own passphrase prompts are connected directly to
// the caller's terminal — this process never sees the passphrase.
func (c *SSHClient) GenerateKey(path, comment string, withPassphrase, overwrite bool) error {
	if _, err := os.Stat(path); err == nil {
		if !overwrite {
			return app.ErrKeyFileExists
		}
		os.Remove(path)
		os.Remove(path + ".pub")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check existing key at %s: %w", path, err)
	}

	args := []string{"-t", "ed25519", "-C", comment, "-f", path}
	if !withPassphrase {
		args = append(args, "-N", "")
	}

	cmd := exec.Command("ssh-keygen", args...)
	if withPassphrase {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	var stderr bytes.Buffer
	if !withPassphrase {
		cmd.Stderr = &stderr
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ssh-keygen: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// ValidateKey reports whether path is a readable, well-formed private key.
func (c *SSHClient) ValidateKey(path string) error {
	var stderr bytes.Buffer
	cmd := exec.Command("ssh-keygen", "-l", "-f", path)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s is not a valid SSH key: %s", path, firstLine(stderr.String()))
	}
	return nil
}

// PublicKeyFingerprint returns the key's SHA256 fingerprint (RF-07).
func (c *SSHClient) PublicKeyFingerprint(path string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("ssh-keygen", "-l", "-f", path)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s is not a valid SSH key: %s", path, firstLine(stderr.String()))
	}

	// Typical line: "256 SHA256:AAAA...xyz comment (ED25519)"
	fields := strings.Fields(stdout.String())
	for _, f := range fields {
		if strings.HasPrefix(f, "SHA256:") {
			return f, nil
		}
	}
	return "", fmt.Errorf("could not parse fingerprint from ssh-keygen output: %q", stdout.String())
}

// PublicKeyContent returns the public key line for the private key at
// path — the .pub file next to it if present, otherwise derived directly
// from the private key via ssh-keygen -y (which needs no passphrase for
// the OpenSSH key format this client generates; see the package doc).
func (c *SSHClient) PublicKeyContent(path string) (string, error) {
	if data, err := os.ReadFile(path + ".pub"); err == nil {
		return strings.TrimSpace(string(data)), nil
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("ssh-keygen", "-y", "-f", path)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("derive public key for %s: %s", path, firstLine(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// KeyPermissionIssue reports whether path's private key permissions are
// unsafe (§13.10 #3, PRD 2 RF-46). cause=="" means secure; otherwise cause
// describes the problem and fixCommand is the exact command that fixes
// it. Windows has no POSIX mode bits — os.Stat synthesizes a meaningless
// ~0666 for any writable file there — so it's checked via ACLs (icacls)
// instead.
func (c *SSHClient) KeyPermissionIssue(path string) (cause, fixCommand string, err error) {
	if runtime.GOOS == "windows" {
		return windowsKeyPermissionIssue(path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", "", fmt.Errorf("stat %s: %w", path, err)
	}
	perm := info.Mode().Perm()
	if perm&0o077 != 0 {
		return fmt.Sprintf("permissions are too open (%04o); private keys should be 600", perm),
			fmt.Sprintf("chmod 600 %s", path), nil
	}
	return "", "", nil
}

// windowsBroadIdentities are well-known Windows groups that, if granted
// access to a private key, make it readable by more than just its owner —
// the ACL equivalent of a POSIX group/other bit being set.
//
// Known limitation: these are the English display names icacls uses;
// this check doesn't attempt to handle other Windows UI languages, or
// broad access granted through a differently-named group (e.g. a
// corporate AD group).
var windowsBroadIdentities = []string{"Everyone", `BUILTIN\Users`, "Authenticated Users"}

func windowsKeyPermissionIssue(path string) (cause, fixCommand string, err error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("icacls", path)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if runErr := cmd.Run(); runErr != nil {
		// icacls writes some failures to stdout rather than stderr.
		msg := firstLine(stderr.String())
		if msg == "" {
			msg = firstLine(stdout.String())
		}
		return "", "", fmt.Errorf("icacls %s: %s", path, msg)
	}

	for _, line := range strings.Split(stdout.String(), "\n") {
		// "(I)" marks an inherited ACE — not something specific to this
		// key, so not this check's business — and "(DENY)" is a
		// hardening measure, not a grant; neither counts as "too open".
		if strings.Contains(line, "(I)") || strings.Contains(line, "(DENY)") {
			continue
		}
		for _, identity := range windowsBroadIdentities {
			if strings.Contains(line, identity) {
				// One icacls call, not several chained with "&&": /reset
				// clears every explicit ACE first (including whichever
				// broad grant was just detected — /grant:r alone only
				// replaces the *named* trustee's own entry, leaving
				// others untouched), /inheritance:r drops what inheriting
				// then re-adds, and /grant:r leaves the current user with
				// sole access. %USERNAME% is resolved here rather than
				// left for the shell to expand, since PowerShell (the
				// Windows default) doesn't do %VAR% expansion at all.
				user := os.Getenv("USERNAME")
				return fmt.Sprintf("permissions are too open: %q has explicit access", identity),
					fmt.Sprintf(`icacls "%s" /reset /inheritance:r /grant:r "%s":F`, path, user), nil
			}
		}
	}
	return "", "", nil
}

// ScanKeys lists usable private keys in dir (RF-40, RF-42): every file
// whose name ends in ".pub" that has a matching private key next to it
// (same name, without the suffix) and validates. A missing dir yields no
// candidates, not an error — a fresh install has no ~/.ssh yet. Anything
// that doesn't validate is silently skipped: it isn't "a key" as far as
// the selector is concerned, and RF-42 doesn't ask to report it.
//
// Validity is read off PublicKeyFingerprint's own error rather than also
// calling ValidateKey first: both run the same `ssh-keygen -l -f` under
// the hood, so calling both would shell out twice per candidate for no
// extra information.
func (c *SSHClient) ScanKeys(dir string) ([]domain.SSHKeyCandidate, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}

	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	sort.Strings(names)

	var candidates []domain.SSHKeyCandidate
	for _, name := range names {
		if !strings.HasSuffix(name, ".pub") {
			continue
		}
		pub := filepath.Join(dir, name)
		priv := strings.TrimSuffix(pub, ".pub")
		if _, err := os.Stat(priv); err != nil {
			continue
		}
		fp, err := c.PublicKeyFingerprint(priv)
		if err != nil {
			continue
		}
		candidates = append(candidates, domain.SSHKeyCandidate{
			PrivateKeyPath: priv,
			PublicKeyPath:  pub,
			Fingerprint:    fp,
		})
	}
	return candidates, nil
}

// DeleteKey removes the private key at path and its ".pub" file (RF-08).
// A missing .pub file is not an error — the private key is still deleted.
func (c *SSHClient) DeleteKey(path string) error {
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete %s: %w", path, err)
	}
	if err := os.Remove(path + ".pub"); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete %s.pub: %w", path, err)
	}
	return nil
}

// KeyHasPassphrase reports whether path is encrypted, without decrypting
// it: ssh-keygen -y with an empty passphrase either succeeds (no
// passphrase) or fails with "incorrect passphrase" (has one).
func (c *SSHClient) KeyHasPassphrase(path string) (bool, error) {
	var stderr bytes.Buffer
	cmd := exec.Command("ssh-keygen", "-y", "-f", path, "-P", "")
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return false, nil
	}
	msg := stderr.String()
	if strings.Contains(msg, "incorrect passphrase") || strings.Contains(msg, "bad passphrase") {
		return true, nil
	}
	return false, fmt.Errorf("%s: %s", path, firstLine(msg))
}

var authenticatedUserPattern = regexp.MustCompile(`Hi ([^!]+)!`)

// TestConnectionWithKey forces the given key and reports whether GitHub
// accepted it. Per §4.5, `ssh -T` always exits 1 even on success, so
// success is read from the output, never the exit code.
func (c *SSHClient) TestConnectionWithKey(host, keyPath string) (domain.AuthResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ssh",
		"-T",
		"-i", keyPath,
		"-o", "IdentitiesOnly=yes",
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=accept-new",
		fmt.Sprintf("git@%s", host),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return domain.AuthResult{}, fmt.Errorf("run ssh: %w", err)
		}
		// A non-zero exit is expected even on success (§4.5); it also
		// covers real auth failures, which we report via RawOutput below.
	}

	return parseAuthResult(string(out)), nil
}

// parseAuthResult is the §4.5 rule as a pure function: success is read from
// the text ("Hi <user>! You've successfully authenticated"), never from the
// exit code. Kept separate from TestConnectionWithKey so it can be unit
// tested against fixture output without a network connection.
func parseAuthResult(output string) domain.AuthResult {
	match := authenticatedUserPattern.FindStringSubmatch(output)
	if match == nil {
		return domain.AuthResult{Success: false, RawOutput: output}
	}
	return domain.AuthResult{Success: true, Username: match[1], RawOutput: output}
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
