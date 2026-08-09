package domain_test

import (
	"testing"

	"github.com/lean-tech/git-rodolfo/internal/domain"
)

func TestParseRepositoryURL(t *testing.T) {
	tests := []struct {
		raw       string
		wantHost  string
		wantOwner string
		wantRepo  string
		wantSSH   bool
	}{
		{"git@github.com:lean-tech/project.git", "github.com", "lean-tech", "project", true},
		{"git@github.com:lean-tech/project", "github.com", "lean-tech", "project", true},
		{"https://github.com/lean-tech/project.git", "github.com", "lean-tech", "project", false},
		{"https://github.com/lean-tech/project", "github.com", "lean-tech", "project", false},
		{"ssh://git@github.com/lean-tech/project.git", "github.com", "lean-tech", "project", true},
		{"git@gitlab.com:team/project.git", "gitlab.com", "team", "project", true},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got, err := domain.ParseRepositoryURL(tt.raw)
			if err != nil {
				t.Fatalf("ParseRepositoryURL(%q): %v", tt.raw, err)
			}
			if got.Host != tt.wantHost || got.Owner != tt.wantOwner || got.Repo != tt.wantRepo {
				t.Fatalf("got %+v, want host=%s owner=%s repo=%s", got, tt.wantHost, tt.wantOwner, tt.wantRepo)
			}
			wantScheme := domain.RepositoryURLSchemeHTTPS
			if tt.wantSSH {
				wantScheme = domain.RepositoryURLSchemeSSH
			}
			if got.Scheme != wantScheme {
				t.Fatalf("scheme = %v, want %v", got.Scheme, wantScheme)
			}
		})
	}
}

func TestParseRepositoryURL_Invalid(t *testing.T) {
	for _, raw := range []string{"", "not a url at all", "https://github.com/onlyowner", "git@github.com:onlyowner"} {
		if _, err := domain.ParseRepositoryURL(raw); err == nil {
			t.Fatalf("expected an error parsing %q", raw)
		}
	}
}

// TestParseRepositoryURL_RejectsLeadingDashInOwnerOrRepo is a regression
// test: Repo is used verbatim as a local clone destination
// (CloneService.Clone), so a repo/owner segment starting with "-" could be
// parsed by `git clone` as an option (e.g. "--template=<attacker-dir>")
// instead of a path — a known git argument-injection class. This is
// rejected here in addition to the "--" separator CloneWithSSHCommand adds,
// as defense in depth.
func TestParseRepositoryURL_RejectsLeadingDashInOwnerOrRepo(t *testing.T) {
	for _, raw := range []string{
		"git@github.com:lean-tech/--template=evil.git",
		"git@github.com:-lean-tech/project.git",
		"https://github.com/lean-tech/--upload-pack=evil",
	} {
		if _, err := domain.ParseRepositoryURL(raw); err == nil {
			t.Fatalf("expected %q to be rejected", raw)
		}
	}
}

func TestRepositoryURL_String(t *testing.T) {
	ssh := domain.RepositoryURL{Scheme: domain.RepositoryURLSchemeSSH, Host: "github.com", Owner: "lean-tech", Repo: "project"}
	if got := ssh.String(); got != "git@github.com:lean-tech/project.git" {
		t.Fatalf("got %q", got)
	}

	https := domain.RepositoryURL{Scheme: domain.RepositoryURLSchemeHTTPS, Host: "github.com", Owner: "lean-tech", Repo: "project"}
	if got := https.String(); got != "https://github.com/lean-tech/project" {
		t.Fatalf("got %q", got)
	}
}
