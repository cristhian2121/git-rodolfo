package fakes

import "github.com/lean-tech/git-rodolfo/internal/domain"

// FakeProviderClient is an in-memory app.ProviderClient double.
type FakeProviderClient struct {
	AuthResults    map[string]domain.AuthResult // keyed by account ID
	AuthErr        error
	CLIUsername    string
	CLIUsernameErr error
	RegisteredKeys []RegisteredKey
	RegisterKeyErr error
}

// RegisteredKey records a single RegisterPublicKey invocation.
type RegisteredKey struct {
	AccountID string
	PublicKey string
}

// NewFakeProviderClient returns an empty FakeProviderClient ready to use.
func NewFakeProviderClient() *FakeProviderClient {
	return &FakeProviderClient{AuthResults: map[string]domain.AuthResult{}}
}

func (f *FakeProviderClient) VerifyAuthentication(account domain.Account) (domain.AuthResult, error) {
	if f.AuthErr != nil {
		return domain.AuthResult{}, f.AuthErr
	}
	return f.AuthResults[account.ID], nil
}

func (f *FakeProviderClient) ActiveCLIUsername() (string, error) {
	if f.CLIUsernameErr != nil {
		return "", f.CLIUsernameErr
	}
	return f.CLIUsername, nil
}

func (f *FakeProviderClient) RegisterPublicKey(account domain.Account, publicKey string) error {
	if f.RegisterKeyErr != nil {
		return f.RegisterKeyErr
	}
	f.RegisteredKeys = append(f.RegisteredKeys, RegisteredKey{account.ID, publicKey})
	return nil
}
