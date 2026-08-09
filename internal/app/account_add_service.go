package app

import (
	"errors"
	"fmt"
	"time"

	"github.com/lean-tech/git-rodolfo/internal/domain"
)

// KeyMode selects how account add obtains an SSH key (PRD §13.2).
type KeyMode int

const (
	KeyModeConfigureLater KeyMode = iota
	KeyModeGenerate
	KeyModeExisting
)

// KeyAlreadyInUseError is CU-02: the key's fingerprint already belongs to
// another registered account. GitHub rejects the same public key on two
// accounts (§4.5), so Git Rodolfo refuses it up front instead.
type KeyAlreadyInUseError struct {
	KeyPath          string
	OtherAccountName string
	RequestedForName string
}

func (e *KeyAlreadyInUseError) Error() string {
	return fmt.Sprintf("key %s is already used by account %q", e.KeyPath, e.OtherAccountName)
}

// ErrAccountAlreadyExists is returned when the derived account ID collides
// with one already registered.
var ErrAccountAlreadyExists = errors.New("account already exists")

// ErrUnsupportedProvider is RF-25: only GitHub is accepted in the MVP.
var ErrUnsupportedProvider = errors.New("git-rodolfo only supports GitHub in this version")

// AccountAddInput carries every value the interactive wizard or
// --non-interactive flags collect for "account add".
type AccountAddInput struct {
	DisplayName      string
	ProviderUsername string
	GitName          string
	GitEmail         string
	// Provider defaults to domain.ProviderGitHub when empty.
	Provider string

	KeyMode KeyMode
	// KeyPath is the target path to write to (KeyModeGenerate) or the
	// existing private key to associate (KeyModeExisting).
	KeyPath        string
	WithPassphrase bool // only meaningful for KeyModeGenerate
	OverwriteKey   bool

	// LoadIntoAgent requests loading a passphrase-protected key into
	// ssh-agent (RF-11); ignored if the key has no passphrase.
	LoadIntoAgent bool
	UseKeychain   bool
}

// AccountAddService implements "account add" (RF-01, RF-06, RF-07, RF-11,
// RF-25). It ends with the account saved as authenticationStatus:
// unverified — real SSH/GitHub verification is wired in by Sprint 3
// (RF-13), which is what finally flips that status.
type AccountAddService struct {
	repo  AccountRepository
	ssh   SSHClient
	agent SSHAgentAdapter
	now   func() time.Time
}

// NewAccountAddService builds an AccountAddService. now defaults to
// time.Now if nil.
func NewAccountAddService(repo AccountRepository, ssh SSHClient, agent SSHAgentAdapter, now func() time.Time) *AccountAddService {
	if now == nil {
		now = time.Now
	}
	return &AccountAddService{repo: repo, ssh: ssh, agent: agent, now: now}
}

// Add validates input, resolves the SSH key (generating or associating one
// as requested), and persists the new account.
func (s *AccountAddService) Add(input AccountAddInput) (domain.Account, error) {
	provider := input.Provider
	if provider == "" {
		provider = domain.ProviderGitHub
	}
	if provider != domain.ProviderGitHub {
		return domain.Account{}, ErrUnsupportedProvider
	}
	if input.DisplayName == "" || input.ProviderUsername == "" || input.GitName == "" || input.GitEmail == "" {
		return domain.Account{}, errors.New("account name, provider username, commit name and commit email are all required")
	}

	id := Slugify(input.DisplayName)
	if id == "" {
		return domain.Account{}, fmt.Errorf("account name %q does not produce a usable id", input.DisplayName)
	}
	if existing, err := s.repo.FindByID(id); err != nil {
		return domain.Account{}, err
	} else if existing != nil {
		return domain.Account{}, fmt.Errorf("%w: %q", ErrAccountAlreadyExists, input.DisplayName)
	}

	now := s.now()
	account := domain.Account{
		ID:                   id,
		DisplayName:          input.DisplayName,
		GitName:              input.GitName,
		GitEmail:             input.GitEmail,
		Provider:             provider,
		ProviderUsername:     input.ProviderUsername,
		Hostname:             "github.com",
		SSHUser:              "git",
		AuthenticationStatus: domain.AuthenticationStatusUnverified,
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	if err := s.resolveKey(&account, input); err != nil {
		return domain.Account{}, err
	}

	if err := s.repo.Save(account); err != nil {
		return domain.Account{}, err
	}
	return account, nil
}

func (s *AccountAddService) resolveKey(account *domain.Account, input AccountAddInput) error {
	if input.KeyMode != KeyModeConfigureLater {
		if err := validateKeyPath(input.KeyPath); err != nil {
			return err
		}
	}

	switch input.KeyMode {
	case KeyModeConfigureLater:
		return nil

	case KeyModeGenerate:
		if err := s.ssh.GenerateKey(input.KeyPath, account.GitEmail, input.WithPassphrase, input.OverwriteKey); err != nil {
			return err
		}
		account.PrivateKeyPath = input.KeyPath
		account.PublicKeyPath = input.KeyPath + ".pub"

	case KeyModeExisting:
		if err := s.ssh.ValidateKey(input.KeyPath); err != nil {
			return err
		}
		account.PrivateKeyPath = input.KeyPath
		account.PublicKeyPath = input.KeyPath + ".pub"

	default:
		return fmt.Errorf("unknown key mode %d", input.KeyMode)
	}

	fp, err := s.ssh.PublicKeyFingerprint(account.PrivateKeyPath)
	if err != nil {
		return err
	}
	if dup, err := s.repo.FindByFingerprint(fp); err != nil {
		return err
	} else if dup != nil {
		return &KeyAlreadyInUseError{
			KeyPath:          account.PrivateKeyPath,
			OtherAccountName: dup.DisplayName,
			RequestedForName: account.DisplayName,
		}
	}
	account.PublicKeyFingerprint = fp

	if input.LoadIntoAgent {
		hasPassphrase, err := s.ssh.KeyHasPassphrase(account.PrivateKeyPath)
		if err != nil {
			return err
		}
		if hasPassphrase {
			if err := s.agent.AddKey(account.PrivateKeyPath, input.UseKeychain); err != nil {
				return err
			}
		}
	}
	return nil
}
