package app

import (
	"fmt"

	"github.com/lean-tech/git-rodolfo/internal/domain"
)

// GitHubCLIMismatchError is RF-14: `gh ssh-key add` registers on whichever
// account GitHub CLI is currently authenticated as, which may not be the
// one being configured. Registering blindly would silently attach the key
// to the wrong GitHub account.
type GitHubCLIMismatchError struct {
	ActiveUsername  string
	AccountUsername string
}

func (e *GitHubCLIMismatchError) Error() string {
	return fmt.Sprintf("GitHub CLI is authenticated as %q, but this account is %q", e.ActiveUsername, e.AccountUsername)
}

// RegisterPublicKeyIfMatchingSession checks gh's active session before
// registering a key (RF-14), refusing with GitHubCLIMismatchError instead
// of registering the key on the wrong account.
func RegisterPublicKeyIfMatchingSession(provider ProviderClient, account domain.Account, publicKey string) error {
	active, err := provider.ActiveCLIUsername()
	if err != nil {
		return err
	}
	if active != account.ProviderUsername {
		return &GitHubCLIMismatchError{ActiveUsername: active, AccountUsername: account.ProviderUsername}
	}
	return provider.RegisterPublicKey(account, publicKey)
}
