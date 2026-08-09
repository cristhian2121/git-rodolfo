package infra

import (
	"os/exec"
	"testing"
)

// These are guarded integration checks: gh's actual auth behavior depends
// on a real, logged-in session, which isn't available in CI or on a
// machine without gh installed (as is the case in this environment). The
// account-matching business logic (RF-14) is covered without gh at all by
// app.RegisterPublicKeyIfMatchingSession's tests against a fake
// ProviderClient; this file only exercises that the adapter shells out
// without crashing.
func TestGitHubCLIAdapter_ActiveCLIUsername_NoCrash(t *testing.T) {
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh not found on PATH")
	}
	// Either a username or ErrGitHubCLINotAvailable is acceptable here —
	// this environment may or may not have an authenticated gh session.
	_, _ = NewGitHubCLIAdapter().ActiveCLIUsername()
}

func TestParseGHAuthStatus_Match(t *testing.T) {
	output := "github.com\n  ✓ Logged in to github.com account cristhiandelgado-work (keyring)\n  - Active account: true\n"
	result := parseGHAuthStatus(output, "cristhiandelgado-work")
	if !result.Success || result.Username != "cristhiandelgado-work" {
		t.Fatalf("expected a match, got %+v", result)
	}
}

func TestParseGHAuthStatus_NoMatch(t *testing.T) {
	output := "github.com\n  ✓ Logged in to github.com account cristhiandelgado (keyring)\n"
	result := parseGHAuthStatus(output, "cristhiandelgado-work")
	if result.Success {
		t.Fatalf("expected no match for a different account, got %+v", result)
	}
	if result.RawOutput != output {
		t.Fatal("expected the raw output to be preserved even on no-match")
	}
}

func TestParseGHAuthStatus_NotLoggedIn(t *testing.T) {
	result := parseGHAuthStatus("You are not logged into any GitHub hosts.\n", "cristhiandelgado-work")
	if result.Success {
		t.Fatal("expected no match when not logged in at all")
	}
}
