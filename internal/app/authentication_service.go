package app

import (
	"fmt"
	"time"

	"github.com/lean-tech/git-rodolfo/internal/domain"
)

// VerifyOutcome is the result of testing an account's SSH key against
// GitHub (RF-13).
type VerifyOutcome struct {
	Account domain.Account
	Result  domain.AuthResult
	// UsernameMismatch is §21's "the key authenticates as another user":
	// the SSH handshake succeeded, but the account that accepted it isn't
	// the one this account was registered under.
	UsernameMismatch bool
}

// AuthenticationService implements RF-13: force the account's key, read
// success from the SSH output (never the exit code, §4.5), and compare
// the authenticated user against providerUsername. It always persists the
// resulting status — a failed or mismatched verification is a fact about
// the account, not a reason to refuse recording it.
type AuthenticationService struct {
	ssh  SSHClient
	repo AccountRepository
	now  func() time.Time
}

// NewAuthenticationService builds an AuthenticationService. now defaults
// to time.Now if nil.
func NewAuthenticationService(ssh SSHClient, repo AccountRepository, now func() time.Time) *AuthenticationService {
	if now == nil {
		now = time.Now
	}
	return &AuthenticationService{ssh: ssh, repo: repo, now: now}
}

// Verify tests account's configured key and persists the updated status.
// It returns an error only for an actual execution failure (e.g. the ssh
// binary missing); an authentication failure is reported through
// VerifyOutcome, not err.
func (s *AuthenticationService) Verify(account domain.Account) (VerifyOutcome, error) {
	if account.PrivateKeyPath == "" {
		return VerifyOutcome{}, fmt.Errorf("account %q has no SSH key configured to verify", account.DisplayName)
	}

	result, err := s.ssh.TestConnectionWithKey(account.Hostname, account.PrivateKeyPath)
	if err != nil {
		return VerifyOutcome{}, err
	}

	mismatch := result.Success && result.Username != account.ProviderUsername

	account.LastVerifiedAt = timePtr(s.now())
	account.UpdatedAt = *account.LastVerifiedAt
	if result.Success && !mismatch {
		account.AuthenticationStatus = domain.AuthenticationStatusVerified
	} else {
		account.AuthenticationStatus = domain.AuthenticationStatusFailed
	}

	if err := s.repo.Update(account); err != nil {
		return VerifyOutcome{}, err
	}

	return VerifyOutcome{Account: account, Result: result, UsernameMismatch: mismatch}, nil
}

func timePtr(t time.Time) *time.Time { return &t }
