package cli_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/cli"
	"github.com/lean-tech/git-rodolfo/internal/domain"
	"github.com/lean-tech/git-rodolfo/internal/infra/fakes"
)

func fixedNow() time.Time {
	return time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
}

func newDeps(accounts ...domain.Account) (cli.Deps, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	repo := fakes.NewFakeAccountRepository(accounts...)
	return cli.Deps{
		Accounts: app.NewAccountService(repo),
		Stdout:   &stdout,
		Stderr:   &stderr,
		Now:      fixedNow,
	}, &stdout, &stderr
}

func TestRun_AccountsListsRegisteredAccounts(t *testing.T) {
	deps, stdout, stderr := newDeps(
		domain.Account{ID: "personal", DisplayName: "Personal", GitEmail: "cristhian@gmail.com", AuthenticationStatus: domain.AuthenticationStatusVerified},
		domain.Account{ID: "lean-tech", DisplayName: "Lean Tech", GitEmail: "cristhian@leantech.com", AuthenticationStatus: domain.AuthenticationStatusVerified},
		domain.Account{ID: "university", DisplayName: "University", GitEmail: "cd@university.edu", AuthenticationStatus: domain.AuthenticationStatusFailed},
	)

	code := cli.Run([]string{"accounts"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"Git accounts:",
		"Personal", "cristhian@gmail.com", "✓ verified",
		"Lean Tech", "cristhian@leantech.com",
		"University", "cd@university.edu", "✗ authentication failed",
		"3 accounts. Run \"git rodolfo account show <account>\" for details.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n---\n%s", want, out)
		}
	}
}

func TestRun_AccountsEmpty(t *testing.T) {
	deps, stdout, _ := newDeps()

	code := cli.Run([]string{"accounts"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stdout.String(), "No accounts registered.") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
}

func TestRun_AccountShow(t *testing.T) {
	verifiedAt := fixedNow().Add(-2 * time.Hour)
	deps, stdout, _ := newDeps(domain.Account{
		ID:                   "lean-tech",
		DisplayName:          "Lean Tech",
		Provider:             domain.ProviderGitHub,
		ProviderUsername:     "cristhiandelgado-work",
		GitName:              "Cristhian Delgado",
		GitEmail:             "cristhian@leantech.com",
		PrivateKeyPath:       "~/.ssh/id_ed25519_leantech",
		AuthenticationStatus: domain.AuthenticationStatusVerified,
		LastVerifiedAt:       &verifiedAt,
	})

	code := cli.Run([]string{"account", "show", "lean-tech"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	want := "Account: Lean Tech\n" +
		"Provider: GitHub\n" +
		"Username: cristhiandelgado-work\n" +
		"Commit name: Cristhian Delgado\n" +
		"Commit email: cristhian@leantech.com\n" +
		"SSH key: ~/.ssh/id_ed25519_leantech\n" +
		"Authentication: verified 2 hours ago\n"
	if stdout.String() != want {
		t.Fatalf("got:\n%s\nwant:\n%s", stdout.String(), want)
	}
}

func TestRun_AccountShowStaleVerification(t *testing.T) {
	verifiedAt := fixedNow().Add(-10 * 24 * time.Hour)
	deps, stdout, _ := newDeps(domain.Account{
		ID:                   "lean-tech",
		DisplayName:          "Lean Tech",
		AuthenticationStatus: domain.AuthenticationStatusVerified,
		LastVerifiedAt:       &verifiedAt,
	})

	code := cli.Run([]string{"account", "show", "lean-tech"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stdout.String(), "not verified recently (last verified 10 days ago)") {
		t.Fatalf("expected stale verification message, got:\n%s", stdout.String())
	}
}

func TestRun_AccountShowNotFound(t *testing.T) {
	deps, _, stderr := newDeps()

	code := cli.Run([]string{"account", "show", "ghost"}, deps)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `no account named "ghost"`) {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRun_AccountShowMissingArgument(t *testing.T) {
	deps, _, stderr := newDeps()

	code := cli.Run([]string{"account", "show"}, deps)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "requires an account id") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	deps, _, stderr := newDeps()

	code := cli.Run([]string{"bogus"}, deps)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `unknown command "bogus"`) {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRun_NoArgsShowsHelp(t *testing.T) {
	deps, stdout, _ := newDeps()

	code := cli.Run(nil, deps)

	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("expected help text, got: %s", stdout.String())
	}
}

func TestRun_HelpAndVersion(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"--help"}, {"-h"}} {
		deps, stdout, _ := newDeps()
		if code := cli.Run(args, deps); code != 0 {
			t.Fatalf("%v: exit code = %d", args, code)
		}
		if !strings.Contains(stdout.String(), "Usage:") {
			t.Fatalf("%v: expected help text, got: %s", args, stdout.String())
		}
	}

	deps, stdout, _ := newDeps()
	if code := cli.Run([]string{"--version"}, deps); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stdout.String(), cli.Version) {
		t.Fatalf("expected version %q in output, got: %s", cli.Version, stdout.String())
	}
}

// TestRun_GlobalFlagsCanAppearAnywhere covers RF-12: global flags parse
// without interfering with the subcommand, whichever position they're in.
func TestRun_GlobalFlagsCanAppearAnywhere(t *testing.T) {
	deps, stdout, stderr := newDeps(domain.Account{ID: "lean-tech", DisplayName: "Lean Tech"})

	code := cli.Run([]string{"--non-interactive", "account", "show", "lean-tech", "--verbose"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Account: Lean Tech") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
}
