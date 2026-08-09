package domain

import (
	"fmt"
	"net/url"
	"strings"
)

// RepositoryURLScheme is how a repository URL authenticates.
type RepositoryURLScheme string

const (
	RepositoryURLSchemeSSH   RepositoryURLScheme = "ssh"
	RepositoryURLSchemeHTTPS RepositoryURLScheme = "https"
)

// RepositoryURL is a parsed Git remote URL, enough to decide whether it's
// GitHub (RF-25) and, for SSH, to reconstruct it after an owner/repo split.
type RepositoryURL struct {
	Scheme RepositoryURLScheme
	Host   string
	Owner  string
	Repo   string
}

// String reconstructs the canonical form of the URL: scp-style for SSH
// ("git@host:owner/repo.git"), a plain HTTPS URL otherwise.
func (u RepositoryURL) String() string {
	if u.Scheme == RepositoryURLSchemeSSH {
		return fmt.Sprintf("git@%s:%s/%s.git", u.Host, u.Owner, u.Repo)
	}
	return fmt.Sprintf("https://%s/%s/%s", u.Host, u.Owner, u.Repo)
}

// ParseRepositoryURL recognizes the SSH and HTTPS forms GitHub (and only
// GitHub in the MVP) accepts. It doesn't itself reject non-GitHub hosts —
// callers compare Host against ProviderGitHub's hostname (RF-25) so the
// rejection message can quote the exact remote that was given.
func ParseRepositoryURL(raw string) (RepositoryURL, error) {
	switch {
	case strings.HasPrefix(raw, "https://"), strings.HasPrefix(raw, "http://"):
		u, err := url.Parse(raw)
		if err != nil {
			return RepositoryURL{}, fmt.Errorf("could not parse URL %q: %w", raw, err)
		}
		owner, repo, err := splitOwnerRepo(u.Path)
		if err != nil {
			return RepositoryURL{}, err
		}
		return RepositoryURL{Scheme: RepositoryURLSchemeHTTPS, Host: u.Host, Owner: owner, Repo: repo}, nil

	case strings.HasPrefix(raw, "ssh://"):
		u, err := url.Parse(raw)
		if err != nil {
			return RepositoryURL{}, fmt.Errorf("could not parse URL %q: %w", raw, err)
		}
		owner, repo, err := splitOwnerRepo(u.Path)
		if err != nil {
			return RepositoryURL{}, err
		}
		return RepositoryURL{Scheme: RepositoryURLSchemeSSH, Host: u.Host, Owner: owner, Repo: repo}, nil

	case looksLikeSCPStyle(raw):
		at := strings.IndexByte(raw, '@')
		colon := strings.IndexByte(raw, ':')
		host := raw[at+1 : colon]
		owner, repo, err := splitOwnerRepo(raw[colon+1:])
		if err != nil {
			return RepositoryURL{}, err
		}
		return RepositoryURL{Scheme: RepositoryURLSchemeSSH, Host: host, Owner: owner, Repo: repo}, nil

	default:
		return RepositoryURL{}, fmt.Errorf("unrecognized repository URL: %q", raw)
	}
}

func looksLikeSCPStyle(raw string) bool {
	at := strings.IndexByte(raw, '@')
	colon := strings.IndexByte(raw, ':')
	return at >= 0 && colon > at && !strings.Contains(raw, "://")
}

func splitOwnerRepo(path string) (owner, repo string, err error) {
	parts := strings.SplitN(strings.Trim(path, "/"), "/", 2)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("could not find an owner/repository pair in %q", path)
	}
	owner, repo = parts[0], strings.TrimSuffix(parts[1], ".git")
	// A repo name is also used verbatim as a local clone destination
	// (see CloneService.Clone); rejecting a leading "-" here closes that
	// off as a git argument-injection vector even before the "--"
	// separator in CloneWithSSHCommand, as defense in depth.
	if strings.HasPrefix(owner, "-") || strings.HasPrefix(repo, "-") {
		return "", "", fmt.Errorf("owner/repository names starting with %q are not allowed: %q", "-", path)
	}
	return owner, repo, nil
}
