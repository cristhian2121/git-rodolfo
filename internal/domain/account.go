// Package domain contains the core data model of Git Rodolfo (PRD §18),
// independent of persistence, Git, SSH or GitHub concerns.
package domain

import "time"

// AuthenticationStatus is the last known state of an Account's SSH
// authentication against its provider (PRD §18).
type AuthenticationStatus string

const (
	AuthenticationStatusUnverified AuthenticationStatus = "unverified"
	AuthenticationStatusVerified   AuthenticationStatus = "verified"
	AuthenticationStatusFailed     AuthenticationStatus = "failed"
)

// Account is a locally registered identity, linking a provider account to
// commit identity and an SSH key (PRD §18).
type Account struct {
	ID                   string               `json:"id"`
	DisplayName          string               `json:"displayName"`
	GitName              string               `json:"gitName"`
	GitEmail             string               `json:"gitEmail"`
	Provider             string               `json:"provider"`
	ProviderUsername     string               `json:"providerUsername"`
	Hostname             string               `json:"hostname"`
	SSHUser              string               `json:"sshUser"`
	PrivateKeyPath       string               `json:"privateKeyPath"`
	PublicKeyPath        string               `json:"publicKeyPath"`
	PublicKeyFingerprint string               `json:"publicKeyFingerprint"`
	AuthenticationStatus AuthenticationStatus `json:"authenticationStatus"`
	LastVerifiedAt       *time.Time           `json:"lastVerifiedAt"`
	CreatedAt            time.Time            `json:"createdAt"`
	UpdatedAt            time.Time            `json:"updatedAt"`
}

// AuthResult is the outcome of testing an SSH connection with a specific
// key (PRD §19.3). Success must be read from the SSH output, never from
// the process exit code (PRD §4.5).
type AuthResult struct {
	Success   bool
	Username  string
	RawOutput string
}
