// Package fakes provides in-memory test doubles for every port in
// internal/app, so business logic can be tested without invoking real Git,
// SSH or GitHub commands (PRD RNF-09).
package fakes

import (
	"github.com/lean-tech/git-rodolfo/internal/app"
)

// FakeGitClient is an in-memory app.GitClient double scoped to a single
// repository, matching how the real adapter is used (one instance bound to
// one working directory).
type FakeGitClient struct {
	IsRepo      bool
	LocalConfig map[string]string
	// GlobalConfig backs GetEffectiveConfig's fallback when a key isn't
	// set locally, standing in for what real Git's config precedence
	// would resolve to.
	GlobalConfig map[string]string
	RemoteURLs   map[string]string
	CloneErr     error
	ClonedCalls  []ClonedCall
}

// ClonedCall records a single CloneWithSSHCommand invocation for assertions.
type ClonedCall struct {
	RemoteURL, Destination, SSHCommand string
}

// NewFakeGitClient returns an empty FakeGitClient ready to use.
func NewFakeGitClient() *FakeGitClient {
	return &FakeGitClient{
		LocalConfig:  map[string]string{},
		GlobalConfig: map[string]string{},
		RemoteURLs:   map[string]string{},
	}
}

func (f *FakeGitClient) IsRepository(path string) bool {
	return f.IsRepo
}

func (f *FakeGitClient) CloneWithSSHCommand(remoteURL, destination, sshCommand string) error {
	f.ClonedCalls = append(f.ClonedCalls, ClonedCall{remoteURL, destination, sshCommand})
	if f.CloneErr != nil {
		return f.CloneErr
	}
	f.IsRepo = true
	return nil
}

func (f *FakeGitClient) SetLocalConfig(key, value string) error {
	f.LocalConfig[key] = value
	return nil
}

func (f *FakeGitClient) GetLocalConfig(key string) (string, error) {
	v, ok := f.LocalConfig[key]
	if !ok {
		return "", app.ErrConfigKeyNotSet
	}
	return v, nil
}

func (f *FakeGitClient) GetEffectiveConfig(key string) (string, error) {
	if v, ok := f.LocalConfig[key]; ok {
		return v, nil
	}
	if v, ok := f.GlobalConfig[key]; ok {
		return v, nil
	}
	return "", app.ErrConfigKeyNotSet
}

func (f *FakeGitClient) UnsetLocalConfig(key string) error {
	delete(f.LocalConfig, key)
	return nil
}

func (f *FakeGitClient) GetRemoteURL(remote string) (string, error) {
	url, ok := f.RemoteURLs[remote]
	if !ok {
		return "", app.ErrNoRemote
	}
	return url, nil
}
